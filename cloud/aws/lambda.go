package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	agTypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	cf "github.com/awslabs/goformation/v7/cloudformation"
	cfApigateway "github.com/awslabs/goformation/v7/cloudformation/apigateway"

	"github.com/spatocode/jerm"
	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/utils"
)

type Lambda struct {
	roleARN           *string
	roleName          string
	policyName        string
	functionHandler   string
	functionArn       *string
	description       string
	AwsConfig         aws.Config
	AwsCreds          aws.Credentials
	config            *config.Config
	cfTemplate        *cf.Template
	defaultMaxRetry   int
	maxWaiterDuration time.Duration
	timeout           int32
}

// NewLambda instantiates a new AWS Lambda platform
func NewLambda(cfg *config.Config) (*Lambda, error) {
	l := &Lambda{
		roleName:          fmt.Sprintf("%s-JermLambdaServiceExecutionRole", cfg.Name),
		policyName:        "jerm-permissions",
		description:       "Jerm Deployment",
		config:            cfg,
		defaultMaxRetry:   3,
		maxWaiterDuration: 20,
		timeout:           30,
	}

	lambdaConfig := &config.Lambda{}
	lambdaConfig.Defaults()

	l.config.Lambda = lambdaConfig
	awsConfig, awsCreds, err := l.getAWSConfig()
	if err != nil {
		return nil, err
	}
	l.AwsConfig, l.AwsCreds = *awsConfig, *awsCreds

	err = l.config.ToJson(jerm.DefaultConfigFile)
	if err != nil {
		return nil, err
	}

	err = l.checkPermissions()
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (l *Lambda) checkPermissions() error {
	role, err := l.getIAMRole()
	if err != nil {
		return err
	}
	l.roleARN = role.Arn

	err = l.ensureIAMRolePolicy()
	if err != nil {
		return err
	}
	return nil
}

// Build builds the deployment package for lambda
func (l *Lambda) Build() (string, error) {
	utils.LogInfo("Building Jerm project for Lambda...")
	handler, err := config.NewPythonConfig().Build(l.config)
	dir := filepath.Dir(handler)
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}

	if l.config.Lambda.Handler == "" {
		err := l.CreateFunctionEntry(handler)
		return dir, err
	}
	return dir, nil
}

func (l *Lambda) getAWSConfig() (*aws.Config, *aws.Credentials, error) {
	msg := fmt.Sprintf("Unable to find an AWS profile. Ensure you set up your AWS before using Jerm. See here for more info %s", awsConfigDocsUrl)
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, nil, errors.New(msg)
	}
	creds, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		return nil, nil, errors.New(msg)
	}
	return &cfg, &creds, nil
}

func (l *Lambda) Logs() {
	fmt.Println("Fetching logs...")
	startTime := int64(time.Millisecond * 100000)
	prevStart := startTime
	for {
		logsEvents, err := l.getLogs(startTime)
		if err != nil {
			slog.Error(err.Error())
			return
		}
		var filteredLogs []cwTypes.FilteredLogEvent
		for _, event := range logsEvents {
			if *event.Timestamp > prevStart {
				filteredLogs = append(filteredLogs, event)
			}
		}
		l.printLogs(filteredLogs)
		if filteredLogs != nil {
			prevStart = *filteredLogs[len(filteredLogs)-1].Timestamp
		}
		time.Sleep(time.Second)
	}
}

func (l *Lambda) printLogs(logs []cwTypes.FilteredLogEvent) {
	for _, log := range logs {
		message := log.Message
		time := time.Unix(*log.Timestamp, 0)
		if strings.Contains(*message, "START RequestId") ||
			strings.Contains(*message, "REPORT RequestId") ||
			strings.Contains(*message, "END RequestId") {
			continue
		}
		fmt.Printf("[%s] %s\n", time, strings.TrimSpace(*message))
	}
}

func (l *Lambda) getLogs(startTime int64) ([]cwTypes.FilteredLogEvent, error) {
	var (
		streamNames []string
		response    *cloudwatchlogs.FilterLogEventsOutput
		logEvents   []cwTypes.FilteredLogEvent
	)

	name := fmt.Sprintf("/aws/lambda/%s", l.config.Name)
	streams, err := l.getLogStreams(name)
	if err != nil {
		var rnfErr *cwTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			err := l.createLogStreams(name)
			if err != nil {
				return nil, err
			}
			return l.getLogs(startTime)
		}
		return nil, err
	}

	for _, stream := range streams {
		streamNames = append(streamNames, *stream.LogStreamName)
	}

	for response == nil || response.NextToken != nil {
		response, err = l.filterLogEvents(name, streamNames, startTime, response)
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

func (l *Lambda) createLogStreams(name string) error {
	client := cloudwatchlogs.NewFromConfig(l.AwsConfig)
	_, err := client.CreateLogGroup(context.TODO(), &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(name),
	})
	return err
}

func (l *Lambda) filterLogEvents(logName string, streamNames []string, startTime int64, logEvents *cloudwatchlogs.FilterLogEventsOutput) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	client := cloudwatchlogs.NewFromConfig(l.AwsConfig)
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
	resp, err := client.FilterLogEvents(context.TODO(), logEventsInput)
	return resp, err
}

func (l *Lambda) getLogStreams(logName string) ([]cwTypes.LogStream, error) {
	client := cloudwatchlogs.NewFromConfig(l.AwsConfig)
	resp, err := client.DescribeLogStreams(context.TODO(), &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logName),
		Descending:   aws.Bool(true),
		OrderBy:      cwTypes.OrderByLastEventTime,
	})
	if err != nil {
		return nil, err
	}
	return resp.LogStreams, err
}

func (l *Lambda) Deploy(zipPath string) (bool, error) {
	deployed, err := l.isAlreadyDeployed()
	if err != nil {
		return false, err
	}
	if deployed {
		return true, nil
	}

	l.uploadFileToS3(zipPath)
	functionArn, err := l.createLambdaFunction(zipPath)
	if err != nil {
		return false, err
	}
	l.functionArn = functionArn

	err = l.waitTillFunctionBecomesActive()
	if err != nil {
		return false, err
	}

	l.scheduleEvents()
	err = l.setupApiGateway()
	if err != nil {
		return false, err
	}

	err = l.removeLocalFile(zipPath)
	if err != nil {
		return false, err
	}

	err = l.removeFileFromS3(zipPath)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (l *Lambda) waitTillFunctionBecomesActive() error {
	client := lambda.NewFunctionActiveV2Waiter(lambda.NewFromConfig(l.AwsConfig))
	err := client.Wait(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(l.config.Name),
	}, time.Second*l.maxWaiterDuration)
	if err != nil {
		return err
	}
	return nil
}

func (l *Lambda) waitTillFunctionBecomesUpdated() {
	client := lambda.NewFunctionUpdatedV2Waiter(lambda.NewFromConfig(l.AwsConfig))
	err := client.Wait(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(l.config.Name),
	}, time.Second*l.maxWaiterDuration)
	if err != nil {
		fmt.Println(err)
	}
}

func (l *Lambda) scheduleEvents() {

}

func (l *Lambda) removeLocalFile(zipPath string) error {
	err := os.Remove(zipPath)
	if err != nil {
		return err
	}
	return nil
}

func (l *Lambda) removeFileFromS3(zipPath string) error {
	client := s3.NewFromConfig(l.AwsConfig)
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(l.config.Bucket),
	})
	if err != nil {
		return err
	}

	_, err = client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(l.config.Bucket),
		Key:    aws.String(zipPath),
	})
	if err != nil {
		return err
	}
	return nil
}

func (l *Lambda) Update(zipPath string) error {
	err := l.uploadFileToS3(zipPath)
	if err != nil {
		return err
	}

	file, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	_, err = l.getLambdaFunction(l.config.Name)
	if err != nil {
		return err
	}

	functionArn, err := l.updateLambdaFunction(content)
	if err != nil {
		return err
	}

	l.functionArn = functionArn
	l.waitTillFunctionBecomesUpdated()
	err = l.setupApiGateway()
	if err != nil {
		return err
	}

	err = l.removeLocalFile(zipPath)
	if err != nil {
		return err
	}

	err = l.removeFileFromS3(zipPath)
	if err != nil {
		return err
	}

	return nil
}

func (l *Lambda) Undeploy() error {
	deployed, err := l.isAlreadyDeployed()
	if err != nil {
		return err
	}
	if !deployed {
		msg := "can't find a deployed project. Run 'jerm deploy' to deploy instead"
		return errors.New(msg)
	}

	fmt.Println("Undeploying...")
	err = l.deleteAPIGateway()
	if err != nil {
		return err
	}

	l.deleteAPIGatewayLogs()
	if err != nil {
		return err
	}

	l.deleteLambdaFunction()
	groupName := fmt.Sprintf("/aws/lambda/%s", l.config.Name)
	l.deleteLogGroup(groupName)

	return nil
}

func (l *Lambda) deleteLambdaFunction() {
	fmt.Println("Deleting lambda function...")
	client := lambda.NewFromConfig(l.AwsConfig)
	client.DeleteFunction(context.TODO(), &lambda.DeleteFunctionInput{
		FunctionName: aws.String(l.config.Name),
	})
}

func (l *Lambda) deleteAPIGatewayLogs() error {
	fmt.Println("Deleting API Gateway logs...")
	apiIds, err := l.getRestApis()
	if err != nil {
		return err
	}
	for _, id := range apiIds {
		client := apigateway.NewFromConfig(l.AwsConfig)
		resp, err := client.GetStages(context.TODO(), &apigateway.GetStagesInput{
			RestApiId: id,
		})
		if err != nil {
			return err
		}
		for _, item := range resp.Item {
			groupName := fmt.Sprintf("API-Gateway-Execution-Logs_%s/%s", *id, *item.StageName)
			l.deleteLogGroup(groupName)
		}
	}
	return nil
}

func (l *Lambda) deleteAPIGateway() error {
	fmt.Println("Deleting API Gateway...")
	err := l.deleteStack()
	if err == nil {
		return nil
	}

	apiIds, err := l.getRestApis()
	if err != nil {
		return err
	}
	for _, id := range apiIds {
		client := apigateway.NewFromConfig(l.AwsConfig)
		_, err := client.DeleteRestApi(context.TODO(), &apigateway.DeleteRestApiInput{
			RestApiId: id,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *Lambda) deleteLogGroup(groupName string) {
	client := cloudwatchlogs.NewFromConfig(l.AwsConfig)
	client.DeleteLogGroup(context.TODO(), &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(groupName),
	})
}

func (l *Lambda) Rollback() error {
	var revisions []int
	steps := 1
	response, err := l.listLambdaVersions()
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			msg := "can't find a deployed project. Run 'jerm deploy' to deploy instead"
			return errors.New(msg)
		}
		return err
	}
	if len(response.Versions) > 1 && response.Versions[len(response.Versions)-1].PackageType == "Image" {
		msg := "rollback unavailable for Docker deployment. Aborting"
		return errors.New(msg)
	}

	if len(response.Versions) < steps+1 {
		msg := "invalid revision for rollback. Aborting"
		return errors.New(msg)
	}

	for _, revision := range response.Versions {
		if *revision.Version != "$LATEST" {
			version, _ := strconv.Atoi(*revision.Version)
			revisions = append(revisions, version)
		}
	}
	sort.Slice(revisions, func(i int, j int) bool {
		return revisions[i] > revisions[j]
	})

	name := fmt.Sprintf("function%s:%v", l.config.Name, revisions[steps])
	function, err := l.getLambdaFunction(name)
	if err != nil {
		return err
	}

	location := function.Code.Location
	res, err := utils.Request(*location)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		msg := fmt.Sprintf("Unable to get version %v of code %s", steps, l.config.Name)
		return errors.New(msg)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	l.updateLambdaFunction(b)
	fmt.Println("Done!")
	return nil
}

func (l *Lambda) listLambdaVersions() (*lambda.ListVersionsByFunctionOutput, error) {
	client := lambda.NewFromConfig(l.AwsConfig)
	response, err := client.ListVersionsByFunction(context.TODO(), &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(l.config.Name),
	})
	return response, err
}

func (l *Lambda) CreateFunctionEntry(file string) error {
	utils.LogInfo("Creating lambda handler...")
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	handler := strings.ReplaceAll(awsLambdaHandler, ".wsgi", l.config.Entry+".wsgi")
	_, err = f.Write([]byte(handler))
	if err != nil {
		return err
	}
	l.functionHandler = "handler.lambda_handler"
	return nil
}

func (l *Lambda) getRestApis() ([]*string, error) {
	var apiIds []*string
	apiGatewayClient := apigateway.NewFromConfig(l.AwsConfig)
	resp, err := apiGatewayClient.GetRestApis(context.TODO(), &apigateway.GetRestApisInput{
		Limit: aws.Int32(500),
	})
	for _, item := range resp.Items {
		if *item.Name == l.config.Name {
			apiIds = append(apiIds, item.Id)
		}
	}
	return apiIds, err
}

func (l *Lambda) getApiId() (*string, error) {
	cloudformationClient := cloudformation.NewFromConfig(l.AwsConfig)
	resp, err := cloudformationClient.DescribeStackResource(context.TODO(), &cloudformation.DescribeStackResourceInput{
		StackName:         aws.String(l.config.Name),
		LogicalResourceId: aws.String("Api"),
	})
	if err != nil {
		apiId, err := l.getRestApis()
		if err != nil {
			return nil, err
		}
		if len(apiId) > 0 {
			return apiId[0], err
		}
		return nil, err
	}
	return resp.StackResourceDetail.PhysicalResourceId, err
}

func (l *Lambda) createCFStack() error {
	template := fmt.Sprintf("%s-template-%v.json", l.config.Name, time.Now().Unix())
	data, err := l.cfTemplate.JSON()
	if err != nil {
		return err
	}

	file, err := os.Create(template)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err != nil {
		return err
	}
	l.uploadFileToS3(template)

	url := fmt.Sprintf("https://s3.amazonaws.com/%s/%s", l.config.Bucket, template)
	if l.AwsConfig.Region == "us-gov-west-1" {
		url = fmt.Sprintf("https://s3-us-gov-west-1.amazonaws.com/%s/%s", l.config.Bucket, template)
	}
	client := cloudformation.NewFromConfig(l.AwsConfig)
	_, err = client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(l.config.Name),
	})
	if err != nil {
		fmt.Println("Creating cloud formation stack...")
		tags := []cfTypes.Tag{
			{
				Key:   aws.String("JermProject"),
				Value: aws.String(l.config.Name),
			},
		}
		_, err := client.CreateStack(context.TODO(), &cloudformation.CreateStackInput{
			StackName:    aws.String(l.config.Name),
			TemplateURL:  aws.String(url),
			Tags:         tags,
			Capabilities: make([]cfTypes.Capability, 0),
		})
		if err != nil {
			return err
		}
	} else {
		client.UpdateStack(context.TODO(), &cloudformation.UpdateStackInput{
			StackName:    aws.String(l.config.Name),
			TemplateURL:  aws.String(url),
			Capabilities: make([]cfTypes.Capability, 0),
		})
	}

	for {
		time.Sleep(time.Second * 3)
		resp, _ := client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
			StackName: aws.String(l.config.Name),
		})
		if resp.Stacks == nil {
			continue
		}
		if resp.Stacks[0].StackStatus == cfTypes.StackStatusCreateComplete || resp.Stacks[0].StackStatus == cfTypes.StackStatusUpdateComplete {
			break
		}

		if resp.Stacks[0].StackStatus == cfTypes.StackStatusDeleteComplete ||
			resp.Stacks[0].StackStatus == cfTypes.StackStatusDeleteInProgress ||
			resp.Stacks[0].StackStatus == cfTypes.StackStatusRollbackInProgress ||
			resp.Stacks[0].StackStatus == cfTypes.StackStatusUpdateRollbackCompleteCleanupInProgress ||
			resp.Stacks[0].StackStatus == cfTypes.StackStatusUpdateRollbackComplete {
			msg := "cloudFormation stack creation failed. Please check console"
			return errors.New(msg)
		}
	}
	err = l.removeFileFromS3(template)
	if err != nil {
		return err
	}

	err = l.removeLocalFile(template)
	if err != nil {
		return err
	}

	return nil
}

func (l *Lambda) deleteStack() error {
	client := cloudformation.NewFromConfig(l.AwsConfig)
	resp, err := client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(l.config.Name),
	})
	if err != nil {
		fmt.Printf("Unable to find stack %s\n", l.config.Name)
		return err
	}
	tags := make(map[string]string)
	for _, tag := range resp.Stacks[0].Tags {
		tags[*tag.Key] = *tag.Value
	}
	if tags["JermProject"] == l.config.Name {
		fmt.Println("Deleting cloud formation stack...")
		_, err := client.DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{
			StackName: aws.String(l.config.Name),
		})
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("JermProject not found")
	}
	return nil
}

func (l *Lambda) createMethods(resourceId string, depth int) {
	pre := "aws-us-gov"
	if l.AwsConfig.Region != "us-gov-west-1" {
		pre = "aws"
	}
	integrationUri := fmt.Sprintf("arn:%s:apigateway:%s:lambda:path/2015-03-31/functions/%s/invocations", pre, l.AwsConfig.Region, *l.functionArn)
	methodName := "ANY"
	method := &cfApigateway.Method{}

	method.RestApiId = cf.Ref("Api")
	method.ResourceId = resourceId
	method.HttpMethod = methodName
	method.AuthorizationType = aws.String("NONE")
	method.ApiKeyRequired = aws.Bool(false)
	l.cfTemplate.Resources[fmt.Sprintf("%s%v", methodName, depth)] = method

	method.Integration = &cfApigateway.Method_Integration{
		CacheNamespace:        aws.String("none"),
		Credentials:           l.roleARN,
		IntegrationHttpMethod: aws.String("POST"),
		Type:                  aws.String("AWS_PROXY"),
		PassthroughBehavior:   aws.String("NEVER"),
		Uri:                   &integrationUri,
	}
}

func (l *Lambda) setupApiGateway() error {
	template := cf.NewTemplate()
	template.Description = "Generated automatically by Jerm"
	restApi := &cfApigateway.RestApi{
		Name:        aws.String(l.config.Name),
		Description: aws.String("Automatically created by Jerm"),
	}
	template.Resources["Api"] = restApi

	rootId := cf.GetAtt("Api", "RootResourceId")
	l.cfTemplate = template
	l.createMethods(rootId, 0)

	resource := &cfApigateway.Resource{}
	resource.RestApiId = cf.Ref("Api")
	resource.ParentId = rootId
	resource.PathPart = "{proxy+}"
	l.cfTemplate.Resources["ResourceAnyPathSlashed"] = resource
	l.createMethods(cf.Ref("ResourceAnyPathSlashed"), 1)

	l.createCFStack()

	apiId, err := l.getApiId()
	if err != nil {
		return err
	}

	apiUrl, err := l.deployAPIGateway(apiId)
	if err != nil {
		return err
	}

	fmt.Printf("API Gateway URL: %s", apiUrl)

	return nil
}

func (l *Lambda) deployAPIGateway(apiId *string) (string, error) {
	fmt.Println("Deploying API Gateway...")
	apiGatewayClient := apigateway.NewFromConfig(l.AwsConfig)
	_, err := apiGatewayClient.CreateDeployment(context.TODO(), &apigateway.CreateDeploymentInput{
		StageName:        aws.String(l.config.Stage),
		RestApiId:        apiId,
		Description:      aws.String("Automatically created by Jerm"),
		CacheClusterSize: agTypes.CacheClusterSizeSize0Point5Gb,
	})
	if err != nil {
		msg := fmt.Sprintf("[Deployment Error] %s", err.Error())
		return "", errors.New(msg)
	}

	_, err = apiGatewayClient.UpdateStage(context.TODO(), &apigateway.UpdateStageInput{
		RestApiId: apiId,
		StageName: aws.String(l.config.Stage),
		PatchOperations: []agTypes.PatchOperation{
			{
				Op:    agTypes.OpReplace,
				Path:  aws.String("/*/*/logging/loglevel"),
				Value: aws.String("OFF"),
			},
			{
				Op:    agTypes.OpReplace,
				Path:  aws.String("/*/*/logging/dataTrace"),
				Value: aws.String("false"),
			},
			{
				Op:    agTypes.OpReplace,
				Path:  aws.String("/*/*/metrics/enabled"),
				Value: aws.String("false"),
			},
			{
				Op:    agTypes.OpReplace,
				Path:  aws.String("/*/*/caching/ttlInSeconds"),
				Value: aws.String("300"),
			},
			{
				Op:    agTypes.OpReplace,
				Path:  aws.String("/*/*/caching/dataEncrypted"),
				Value: aws.String("false"),
			},
		},
	})
	if err != nil {
		msg := fmt.Sprintf("[Stage Update Error] %s", err)
		return "", errors.New(msg)
	}

	return fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s\n", *apiId, l.AwsConfig.Region, l.config.Stage), nil
}

func (l *Lambda) createS3Bucket(client *s3.Client, isConfig bool) error {
	if isConfig {
		_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(l.config.Bucket),
			CreateBucketConfiguration: &s3Types.CreateBucketConfiguration{
				LocationConstraint: s3Types.BucketLocationConstraint(l.AwsConfig.Region),
			},
		})
		return err
	}
	_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(l.config.Bucket),
	})
	return err
}

func (l *Lambda) uploadFileToS3(zipPath string) error {
	client := s3.NewFromConfig(l.AwsConfig)
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(l.config.Bucket),
	})
	if err != nil {
		var nfErr *s3Types.NotFound
		if errors.As(err, &nfErr) {
			err := l.createS3Bucket(client, true)
			if err != nil {
				err := l.createS3Bucket(client, false)
				if err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	f, err := os.Stat(zipPath)
	if f.Size() == 0 || err != nil {
		msg := "encountered issue with packaged file"
		return errors.New(msg)
	}

	fmt.Println(zipPath)

	file, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileName := filepath.Base(zipPath)
	utils.LogInfo("Uploading file %s...\n", fileName)
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(l.config.Bucket),
		Key:    aws.String(fileName),
		Body:   file,
	})
	if err != nil {
		msg := "encountered error while uploading package. Aborting"
		return errors.New(msg)
	}
	return nil
}

func (l *Lambda) isAlreadyDeployed() (bool, error) {
	versions, err := l.getLambdaVersionsByFuction()
	if err != nil {
		return false, err
	}
	return len(versions) > 0, nil
}

func (l *Lambda) getLambdaVersionsByFuction() ([]lambdaTypes.FunctionConfiguration, error) {
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.ListVersionsByFunction(context.TODO(), &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(l.config.Name),
	})
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			return nil, nil
		}
		return nil, err
	}
	return resp.Versions, nil
}

func (l *Lambda) createLambdaFunction(zipPath string) (*string, error) {
	name := l.config.Name
	function, err := l.getLambdaFunction(name)
	if err == nil {
		return function.Configuration.FunctionArn, nil
	}
	fileName := filepath.Base(zipPath)
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.CreateFunction(context.TODO(), &lambda.CreateFunctionInput{
		Code: &lambdaTypes.FunctionCode{
			S3Bucket: aws.String(l.config.Bucket),
			S3Key:    aws.String(fileName),
		},
		FunctionName: aws.String(l.config.Name),
		Description:  aws.String(l.description),
		Role:         l.roleARN,
		Runtime:      lambdaTypes.Runtime(l.config.Lambda.Runtime),
		Handler:      aws.String(l.functionHandler),
		Timeout:      aws.Int32(l.timeout),
	})
	if err != nil {
		return nil, err
	}
	return resp.FunctionArn, nil
}

func (l *Lambda) getLambdaFunction(name string) (*lambda.GetFunctionOutput, error) {
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.GetFunction(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(name),
	})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (l *Lambda) updateLambdaFunction(content []byte) (*string, error) {
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.UpdateFunctionCode(context.TODO(), &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(l.config.Name),
		ZipFile:      content,
		Publish:      true,
	})
	if err != nil {
		return nil, err
	}
	return resp.FunctionArn, nil
}

func (l *Lambda) ensureIAMRolePolicy() error {
	slog.Debug("Fetching IAM Role Policy...")
	client := iam.NewFromConfig(l.AwsConfig)
	_, err := client.GetRolePolicy(context.TODO(), &iam.GetRolePolicyInput{
		RoleName:   &l.roleName,
		PolicyName: &l.policyName,
	})
	if err != nil {
		var nseErr *iamTypes.NoSuchEntityException
		if errors.As(err, &nseErr) {
			slog.Debug("IAM Role Policy not found. Creating new IAM Role Policy...")
			_, perr := client.PutRolePolicy(context.TODO(), &iam.PutRolePolicyInput{
				RoleName:       &l.roleName,
				PolicyName:     &l.policyName,
				PolicyDocument: aws.String(awsAttachPolicy),
			})
			if perr != nil {
				return perr
			}
			return nil
		}
		return err
	}
	return nil
}

func (l *Lambda) getIAMRole() (*iamTypes.Role, error) {
	slog.Debug("Fetching IAM Role...")
	client := iam.NewFromConfig(l.AwsConfig)
	resp, err := client.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: &l.roleName,
	})
	if err != nil {
		var nseErr *iamTypes.NoSuchEntityException
		if errors.As(err, &nseErr) {
			slog.Debug("IAM Role not found. Creating new IAM Role ...")
			resp, err := l.createIAMRole()
			return resp.Role, err
		}
		return nil, err
	}

	return resp.Role, nil
}

func (l *Lambda) createIAMRole() (*iam.CreateRoleOutput, error) {
	client := iam.NewFromConfig(l.AwsConfig)
	resp, err := client.CreateRole(context.TODO(), &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(awsAssumePolicy),
		Path:                     aws.String("/"),
		RoleName:                 &l.roleName,
	})
	if err != nil {
		return nil, err
	}

	slog.Debug("Created Role: %s\n", *resp.Role.RoleName)
	return resp, nil
}
