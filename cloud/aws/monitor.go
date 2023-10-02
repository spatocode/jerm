package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
)

const (
	InvocationMetric = "Invocations"
	ErrorMetric = "Errors"
)

// CloudWatch is the AWS Cloudwatch operations
type CloudWatch struct {
	config *config.Config
	log *cloudwatchlogs.Client
	client *cloudwatch.Client
}

// NewCloudWatch creates a new AWS Cloudwatch
func NewCloudWatch(config *config.Config, awsConfig aws.Config) *CloudWatch {
	return &CloudWatch{
		config: config,
		log: cloudwatchlogs.NewFromConfig(awsConfig),
		client: cloudwatch.NewFromConfig(awsConfig),
	}
}

// Watch is an infinite loop that continously fetches AWS Cloudwatch log events
func (c *CloudWatch) Watch() {
	startTime := int64(time.Millisecond * 100000)
	prevStart := startTime
	for {
		logsEvents, err := c.getLogs(startTime)
		if err != nil {
			log.Debug(err.Error())
			return
		}
		var filteredLogs []cwlTypes.FilteredLogEvent
		for _, event := range logsEvents {
			if *event.Timestamp > prevStart {
				filteredLogs = append(filteredLogs, event)
			}
		}
		c.printLogs(filteredLogs)
		if filteredLogs != nil {
			prevStart = *filteredLogs[len(filteredLogs)-1].Timestamp
		}
		time.Sleep(time.Second)
	}
}

// printLogs prints the cloudwatch logs to stdout
func (c *CloudWatch) printLogs(logs []cwlTypes.FilteredLogEvent) {
	for _, l := range logs {
		message := l.Message
		time := time.Unix(*l.Timestamp/1000, 0)
		if strings.Contains(*message, "START RequestId") ||
			strings.Contains(*message, "REPORT RequestId") ||
			strings.Contains(*message, "END RequestId") {
			continue
		}
		log.PrintfInfo("[%s] %s\n", time, strings.TrimSpace(*message))
	}
}

// getLogs gets the list of log streams. It parses and filters the necessary logs streams
func (c *CloudWatch) getLogs(startTime int64) ([]cwlTypes.FilteredLogEvent, error) {
	var (
		streamNames []string
		response    *cloudwatchlogs.FilterLogEventsOutput
		logEvents   []cwlTypes.FilteredLogEvent
	)

	name := fmt.Sprintf("/aws/lambda/%s", c.config.GetFunctionName())
	streams, err := c.getLogStreams(name)
	if err != nil {
		var rnfErr *cwlTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			err := c.createLogStreams(name)
			if err != nil {
				return nil, err
			}
			return c.getLogs(startTime)
		}
		return nil, err
	}

	for _, stream := range streams {
		streamNames = append(streamNames, *stream.LogStreamName)
	}

	for response == nil || response.NextToken != nil {
		response, err = c.filterLogEvents(name, streamNames, startTime, response)
		if err != nil {
			return nil, err
		}
		logEvents = append(logEvents, response.Events...)
	}
	sort.Slice(logEvents, func(i int, j int) bool {
		return *logEvents[i].Timestamp < *logEvents[j].Timestamp
	})
	return logEvents, nil
}

// createLogStreams creates a log group with the specified name
func (c *CloudWatch) createLogStreams(name string) error {
	_, err := c.log.CreateLogGroup(context.TODO(), &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(name),
	})
	return err
}

// filterLogEvents filters the necessary log events
func (c *CloudWatch) filterLogEvents(logName string, streamNames []string, startTime int64, logEvents *cloudwatchlogs.FilterLogEventsOutput) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	logEventsInput := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   aws.String(logName),
		LogStreamNames: streamNames,
		StartTime:      aws.Int64(startTime),
		EndTime:        aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
		Limit:          aws.Int32(10000),
		Interleaved:    aws.Bool(true),
	}
	if logEvents != nil && logEvents.NextToken != nil {
		logEventsInput.NextToken = logEvents.NextToken
	}
	resp, err := c.log.FilterLogEvents(context.TODO(), logEventsInput)
	return resp, err
}

// getLogStreams fetches the list of log streams for the specified log group name
func (c *CloudWatch) getLogStreams(logName string) ([]cwlTypes.LogStream, error) {
	resp, err := c.log.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logName),
		Descending:   aws.Bool(true),
		OrderBy:      cwlTypes.OrderByLastEventTime,
	})
	if err != nil {
		return nil, err
	}
	return resp.LogStreams, err
}

// deleteLogGroup deletes a specified log group name
func (c *CloudWatch) deleteLogGroup(groupName string) error {
	_, err := c.log.DeleteLogGroup(context.TODO(), &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(groupName),
	})
	return err
}

// Clear deletes AWS CloudWatch logs
func (c *CloudWatch) Clear(name string) error {
	err := c.deleteLogGroup(name)
	return err
}

func (c *CloudWatch) getMetrics(name string) (*cloudwatch.GetMetricStatisticsOutput, error) {
	startTime := time.Now().UTC().Add(-24 * time.Hour)
	stats, err := c.client.GetMetricStatistics(context.TODO(), &cloudwatch.GetMetricStatisticsInput{
		Namespace: aws.String("AWS/Lambda"),
		MetricName: aws.String(name),
		StartTime: aws.Time(startTime),
		EndTime: aws.Time(time.Now().UTC()),
		Period: aws.Int32(1440),
		Statistics: []cwTypes.Statistic{cwTypes.StatisticSum},
		Dimensions: []cwTypes.Dimension{
			{
				Name: aws.String("FunctionName"),
				Value: aws.String(c.config.GetFunctionName()),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// Metrics shows CloudWatch metrics
func (c *CloudWatch) Metrics() error {
	res, err := c.getMetrics(InvocationMetric)
	if err != nil {
		return err
	}
	functionInvocations := res.Datapoints[0].Sum

	res, err = c.getMetrics(ErrorMetric)
	if err != nil {
		return err
	}
	functionErrors := res.Datapoints[0].Sum
	errorRate := *functionErrors / *functionInvocations * 100

	return err
}
