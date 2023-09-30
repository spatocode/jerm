package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
)

// CloudWatch is the AWS Cloudwatch operations
type CloudWatch struct {
	config *config.Config
	client *cloudwatchlogs.Client
}

// NewCloudWatch creates a new AWS Cloudwatch
func NewCloudWatch(config *config.Config, awsConfig aws.Config) *CloudWatch {
	return &CloudWatch{
		config: config,
		client: cloudwatchlogs.NewFromConfig(awsConfig),
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
		var filteredLogs []cwTypes.FilteredLogEvent
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
func (c *CloudWatch) printLogs(logs []cwTypes.FilteredLogEvent) {
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
func (c *CloudWatch) getLogs(startTime int64) ([]cwTypes.FilteredLogEvent, error) {
	var (
		streamNames []string
		response    *cloudwatchlogs.FilterLogEventsOutput
		logEvents   []cwTypes.FilteredLogEvent
	)

	name := fmt.Sprintf("/aws/lambda/%s", c.config.GetFunctionName())
	streams, err := c.getLogStreams(name)
	if err != nil {
		var rnfErr *cwTypes.ResourceNotFoundException
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
	_, err := c.client.CreateLogGroup(context.TODO(), &cloudwatchlogs.CreateLogGroupInput{
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
	resp, err := c.client.FilterLogEvents(context.TODO(), logEventsInput)
	return resp, err
}

// getLogStreams fetches the list of log streams for the specified log group name
func (c *CloudWatch) getLogStreams(logName string) ([]cwTypes.LogStream, error) {
	resp, err := c.client.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logName),
		Descending:   aws.Bool(true),
		OrderBy:      cwTypes.OrderByLastEventTime,
	})
	if err != nil {
		return nil, err
	}
	return resp.LogStreams, err
}

// deleteLogGroup deletes a specified log group name
func (c *CloudWatch) deleteLogGroup(groupName string) error {
	_, err := c.client.DeleteLogGroup(context.TODO(), &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(groupName),
	})
	return err
}

// Clear deletes AWS CloudWatch logs
func (c *CloudWatch) Clear() error {
	groupName := fmt.Sprintf("/aws/lambda/%s", c.config.GetFunctionName())
	err := c.deleteLogGroup(groupName)
	return err
}
