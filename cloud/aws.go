package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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

	"github.com/spatocode/bulaba/utils"
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
	config            CloudConfig
	cfTemplate        *cf.Template
	defaultMaxRetry   int
	maxWaiterDuration time.Duration
	timeout           int32
}

func LoadLambda(cfg CloudConfig) *Lambda {
	l := &Lambda{
		roleName:          fmt.Sprintf("%s-BulabaLambdaServiceExecutionRole", cfg.GetFunctionName()),
		policyName:        "bulaba-permissions",
		description:       "Bulaba Deployment",
		config:            cfg,
		defaultMaxRetry:   3,
		maxWaiterDuration: 20,
		timeout:           30,
	}
	l.AwsConfig, l.AwsCreds = l.getAWSConfig()
	return l
}

func (l *Lambda) CheckPermissions() {
	l.roleARN = l.getIAMRole().Arn
	l.ensureIAMRolePolicy()
}

func (l *Lambda) getAWSConfig() (aws.Config, aws.Credentials) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		utils.BulabaException(err.Error())
	}
	creds, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		utils.BulabaException(err.Error())
	}
	return cfg, creds
}

func (l *Lambda) Logs() {
	fmt.Println("Fetching logs...")
	startTime := int64(time.Millisecond * 100000)
	prevStart := startTime
	for {
		logsEvents := l.getLogs(startTime)
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

func (l *Lambda) getLogs(startTime int64) []cwTypes.FilteredLogEvent {
	var (
		streamNames []string
		response    *cloudwatchlogs.FilterLogEventsOutput
		logEvents   []cwTypes.FilteredLogEvent
	)

	name := fmt.Sprintf("/aws/lambda/%s", l.config.GetFunctionName())
	streams, err := l.getLogStreams(name)
	if err != nil {
		var rnfErr *cwTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			err := l.createLogStreams(name)
			if err != nil {
				utils.BulabaException(err)
			}
			return l.getLogs(startTime)
		}
		utils.BulabaException(err)
	}

	for _, stream := range streams {
		streamNames = append(streamNames, *stream.LogStreamName)
	}

	for response == nil || response.NextToken != nil {
		response, err = l.filterLogEvents(name, streamNames, startTime, response)
		if err != nil {
			utils.BulabaException(err)
		}
		logEvents = append(logEvents, response.Events...)
	}
	sort.Slice(logEvents, func(i int, j int) bool {
		return *logEvents[i].Timestamp < *logEvents[j].Timestamp
	})
	return logEvents
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

func (l *Lambda) Deploy(zipPath string) {
	deployed := l.isAlreadyDeployed()
	if deployed {
		l.removeLocalFile(zipPath)
		msg := "Project already deployed. Run 'bulaba update' to update instead"
		utils.BulabaException(msg)
	}
	l.uploadFileToS3(zipPath)
	l.functionArn = l.createLambdaFunction(zipPath)
	l.waitTillFunctionBecomesActive()
	l.scheduleEvents()
	l.setupApiGateway()
	l.removeLocalFile(zipPath)
	l.removeFileFromS3(zipPath)
}

func (l *Lambda) waitTillFunctionBecomesActive() {
	client := lambda.NewFunctionActiveV2Waiter(lambda.NewFromConfig(l.AwsConfig))
	err := client.Wait(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
	}, time.Second*l.maxWaiterDuration)
	if err != nil {
		utils.BulabaException(err)
	}
}

func (l *Lambda) waitTillFunctionBecomesUpdated() {
	client := lambda.NewFunctionUpdatedV2Waiter(lambda.NewFromConfig(l.AwsConfig))
	err := client.Wait(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
	}, time.Second*l.maxWaiterDuration)
	if err != nil {
		fmt.Println(err)
	}
}

func (l *Lambda) scheduleEvents() {

}

func (l *Lambda) removeLocalFile(zipPath string) {
	err := os.Remove(zipPath)
	if err != nil {
		utils.BulabaException(err)
	}
}

func (l *Lambda) removeFileFromS3(zipPath string) {
	client := s3.NewFromConfig(l.AwsConfig)
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(l.config.GetBucket()),
	})
	if err != nil {
		utils.BulabaException(err)
	}

	_, err = client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(l.config.GetBucket()),
		Key:    aws.String(zipPath),
	})
	if err != nil {
		utils.BulabaException(err)
	}
}

func (l *Lambda) Update(zipPath string) {
	l.uploadFileToS3(zipPath)

	file, err := os.Open(zipPath)
	if err != nil {
		utils.BulabaException(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		utils.BulabaException(err)
	}

	_, err = l.getLambdaFunction(l.config.GetFunctionName())
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			msg := "Can't find a deployed project. Run 'bulaba deploy' to deploy instead."
			utils.BulabaException(msg)
		}
		utils.BulabaException(err)
	}

	l.functionArn = l.updateLambdaFunction(content)
	l.waitTillFunctionBecomesUpdated()
	l.setupApiGateway()
	l.removeLocalFile(zipPath)
	l.removeFileFromS3(zipPath)
}

func (l *Lambda) Undeploy() {
	deployed := l.isAlreadyDeployed()
	if !deployed {
		msg := "Can't find a deployed project. Run 'bulaba deploy' to deploy instead."
		utils.BulabaException(msg)
	}
	fmt.Println("Undeploying...")
	l.deleteAPIGateway()
	l.deleteAPIGatewayLogs()
	l.deleteLambdaFunction()
	groupName := fmt.Sprintf("/aws/lambda/%s", l.config.GetFunctionName())
	l.deleteLogGroup(groupName)
}

func (l *Lambda) deleteLambdaFunction() {
	fmt.Println("Deleting lambda function...")
	client := lambda.NewFromConfig(l.AwsConfig)
	client.DeleteFunction(context.TODO(), &lambda.DeleteFunctionInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
	})
}

func (l *Lambda) deleteAPIGatewayLogs() {
	fmt.Println("Deleting API Gateway logs...")
	apiIds, err := l.getRestApis()
	if err != nil {
		utils.BulabaException(err)
	}
	for _, id := range apiIds {
		client := apigateway.NewFromConfig(l.AwsConfig)
		resp, err := client.GetStages(context.TODO(), &apigateway.GetStagesInput{
			RestApiId: id,
		})
		if err != nil {
			utils.BulabaException(err)
		}
		for _, item := range resp.Item {
			groupName := fmt.Sprintf("API-Gateway-Execution-Logs_%s/%s", *id, *item.StageName)
			l.deleteLogGroup(groupName)
		}
	}
}

func (l *Lambda) deleteAPIGateway() {
	fmt.Println("Deleting API Gateway...")
	err := l.deleteStack()
	if err == nil {
		return
	}

	apiIds, err := l.getRestApis()
	if err != nil {
		utils.BulabaException(err)
	}
	for _, id := range apiIds {
		client := apigateway.NewFromConfig(l.AwsConfig)
		_, err := client.DeleteRestApi(context.TODO(), &apigateway.DeleteRestApiInput{
			RestApiId: id,
		})
		if err != nil {
			utils.BulabaException(err)
		}
	}
}

func (l *Lambda) deleteLogGroup(groupName string) {
	client := cloudwatchlogs.NewFromConfig(l.AwsConfig)
	client.DeleteLogGroup(context.TODO(), &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(groupName),
	})
}

func (l *Lambda) Rollback() {
	fmt.Println("Rolling back deployment...")
	var (
		revisions []int
	)
	steps := 1
	response, err := l.listLambdaVersions()
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			msg := "Can't find a deployed project. Run 'bulaba deploy' to deploy instead."
			utils.BulabaException(msg)
		}
		utils.BulabaException(err)
	}
	if len(response.Versions) > 1 && response.Versions[len(response.Versions)-1].PackageType == "Image" {
		utils.BulabaException("Rollback unavailable for Docker deployment. Aborting...")
	}

	if len(response.Versions) < steps+1 {
		utils.BulabaException("Invalid revision for rollback. Aborting...")
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

	name := fmt.Sprintf("function%s:%v", l.config.GetFunctionName(), revisions[steps])
	function, err := l.getLambdaFunction(name)
	if err != nil {
		utils.BulabaException(err)
	}

	location := function.Code.Location
	res, err := utils.Request(*location)
	if err != nil {
		utils.BulabaException(err)
	}

	if res.StatusCode != 200 {
		msg := fmt.Sprintf("Unable to get version %v of code %s", steps, l.config.GetFunctionName())
		utils.BulabaException(msg)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		utils.BulabaException(err)
	}

	l.updateLambdaFunction(b)
	fmt.Println("Done!")
}

func (l *Lambda) listLambdaVersions() (*lambda.ListVersionsByFunctionOutput, error) {
	client := lambda.NewFromConfig(l.AwsConfig)
	response, err := client.ListVersionsByFunction(context.TODO(), &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
	})
	return response, err
}

func (l *Lambda) CreateFunctionEntry(file string) {
	fmt.Println("Creating lambda handler...")
	f, err := os.Create(file)
	if err != nil {
		utils.BulabaException(err.Error())
	}
	defer f.Close()

	projectName := strings.Split(l.config.GetAppsEntry(), ".")[0]
	handler := strings.ReplaceAll(awsLambdaHandler, ".wsgi", projectName+".wsgi")
	_, err = f.Write([]byte(handler))
	if err != nil {
		utils.BulabaException(err.Error())
	}
	l.functionHandler = "handler.lambda_handler"
}

func (l *Lambda) getRestApis() ([]*string, error) {
	var apiIds []*string
	apiGatewayClient := apigateway.NewFromConfig(l.AwsConfig)
	resp, err := apiGatewayClient.GetRestApis(context.TODO(), &apigateway.GetRestApisInput{
		Limit: aws.Int32(500),
	})
	for _, item := range resp.Items {
		if *item.Name == l.config.GetFunctionName() {
			apiIds = append(apiIds, item.Id)
		}
	}
	return apiIds, err
}

func (l *Lambda) getApiId() (*string, error) {
	cloudformationClient := cloudformation.NewFromConfig(l.AwsConfig)
	resp, err := cloudformationClient.DescribeStackResource(context.TODO(), &cloudformation.DescribeStackResourceInput{
		StackName:         aws.String(l.config.GetFunctionName()),
		LogicalResourceId: aws.String("Api"),
	})
	if err != nil {
		apiId, err := l.getRestApis()
		if err != nil {
			utils.BulabaException(err)
		}
		if len(apiId) > 0 {
			return apiId[0], err
		}
		return nil, err
	}
	return resp.StackResourceDetail.PhysicalResourceId, err
}

func (l *Lambda) createCFStack() {
	template := fmt.Sprintf("%s-template-%v.json", l.config.GetFunctionName(), time.Now().Unix())
	data, err := l.cfTemplate.JSON()
	if err != nil {
		utils.BulabaException(err)
	}

	file, err := os.Create(template)
	if err != nil {
		utils.BulabaException(err)
	}
	_, err = file.Write(data)
	if err != nil {
		utils.BulabaException(err.Error())
	}
	l.uploadFileToS3(template)

	url := fmt.Sprintf("https://s3.amazonaws.com/%s/%s", l.config.GetBucket(), template)
	if l.AwsConfig.Region == "us-gov-west-1" {
		url = fmt.Sprintf("https://s3-us-gov-west-1.amazonaws.com/%s/%s", l.config.GetBucket(), template)
	}
	client := cloudformation.NewFromConfig(l.AwsConfig)
	_, err = client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(l.config.GetFunctionName()),
	})
	if err != nil {
		fmt.Println("Creating cloud formation stack...")
		tags := []cfTypes.Tag{
			{
				Key:   aws.String("BulabaProject"),
				Value: aws.String(l.config.GetFunctionName()),
			},
		}
		_, err := client.CreateStack(context.TODO(), &cloudformation.CreateStackInput{
			StackName:    aws.String(l.config.GetFunctionName()),
			TemplateURL:  aws.String(url),
			Tags:         tags,
			Capabilities: make([]cfTypes.Capability, 0),
		})
		if err != nil {
			utils.BulabaException(err)
		}
	} else {
		client.UpdateStack(context.TODO(), &cloudformation.UpdateStackInput{
			StackName:    aws.String(l.config.GetFunctionName()),
			TemplateURL:  aws.String(url),
			Capabilities: make([]cfTypes.Capability, 0),
		})
	}

	for {
		time.Sleep(time.Second * 3)
		resp, _ := client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
			StackName: aws.String(l.config.GetFunctionName()),
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
			utils.BulabaException("CloudFormation stack creation failed. Please check console.")
		}
	}
	l.removeFileFromS3(template)
	l.removeLocalFile(template)
}

func (l *Lambda) deleteStack() error {
	client := cloudformation.NewFromConfig(l.AwsConfig)
	resp, err := client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(l.config.GetFunctionName()),
	})
	if err != nil {
		fmt.Printf("Unable to find stack %s\n", l.config.GetFunctionName())
		return err
	}
	tags := make(map[string]string)
	for _, tag := range resp.Stacks[0].Tags {
		tags[*tag.Key] = *tag.Value
	}
	if tags["BulabaProject"] == l.config.GetFunctionName() {
		fmt.Println("Deleting cloud formation stack...")
		_, err := client.DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{
			StackName: aws.String(l.config.GetFunctionName()),
		})
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("BulabaProject not found")
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

func (l *Lambda) setupApiGateway() {
	template := cf.NewTemplate()
	template.Description = "Generated automatically by Bulaba"
	restApi := &cfApigateway.RestApi{
		Name:        aws.String(l.config.GetFunctionName()),
		Description: aws.String("Automatically created by Bulaba"),
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
		utils.BulabaException(err)
	}
	apiUrl := l.deployAPIGateway(apiId)
	fmt.Printf("API Gateway URL: %s", apiUrl)
}

func (l *Lambda) deployAPIGateway(apiId *string) string {
	fmt.Println("Deploying API Gateway...")
	apiGatewayClient := apigateway.NewFromConfig(l.AwsConfig)
	_, err := apiGatewayClient.CreateDeployment(context.TODO(), &apigateway.CreateDeploymentInput{
		StageName:        aws.String(l.config.GetStage()),
		RestApiId:        apiId,
		Description:      aws.String("Automatically created by Bulaba"),
		CacheClusterSize: agTypes.CacheClusterSizeSize0Point5Gb,
	})
	if err != nil {
		utils.BulabaException("[Deployment Error]", err)
	}

	_, err = apiGatewayClient.UpdateStage(context.TODO(), &apigateway.UpdateStageInput{
		RestApiId: apiId,
		StageName: aws.String(l.config.GetStage()),
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
		utils.BulabaException("[Stage Update Error]", err)
	}

	return fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s\n", *apiId, l.AwsConfig.Region, l.config.GetStage())
}

func (l *Lambda) createS3Bucket(client *s3.Client, isConfig bool) error {
	if isConfig {
		_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(l.config.GetBucket()),
			CreateBucketConfiguration: &s3Types.CreateBucketConfiguration{
				LocationConstraint: s3Types.BucketLocationConstraint(l.AwsConfig.Region),
			},
		})
		return err
	}
	_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(l.config.GetBucket()),
	})
	return err
}

func (l *Lambda) uploadFileToS3(zipPath string) {
	client := s3.NewFromConfig(l.AwsConfig)
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(l.config.GetBucket()),
	})
	if err != nil {
		var nfErr *s3Types.NotFound
		if errors.As(err, &nfErr) {
			err := l.createS3Bucket(client, true)
			if err != nil {
				err := l.createS3Bucket(client, false)
				if err != nil {
					utils.BulabaException(err.Error())
				}
			}
		} else {
			utils.BulabaException(err.Error())
		}
	}

	f, err := os.Stat(zipPath)
	if f.Size() == 0 || err != nil {
		utils.BulabaException("Encountered issue with packaged file.")
	}

	file, err := os.Open(zipPath)
	if err != nil {
		utils.BulabaException(err)
	}
	defer file.Close()

	fileName := filepath.Base(zipPath)
	fmt.Printf("Uploading file %s...\n", fileName)
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(l.config.GetBucket()),
		Key:    aws.String(fileName),
		Body:   file,
	})
	if err != nil {
		utils.BulabaException("Encountered error while uploading package. Aborting...")
	}
}

func (l *Lambda) isAlreadyDeployed() bool {
	versions := l.getLambdaVersionsByFuction()
	return len(versions) > 0
}

func (l *Lambda) getLambdaVersionsByFuction() []lambdaTypes.FunctionConfiguration {
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.ListVersionsByFunction(context.TODO(), &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
	})
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			return nil
		}
		utils.BulabaException(err.Error())
	}
	return resp.Versions
}

func (l *Lambda) createLambdaFunction(zipPath string) *string {
	name := l.config.GetFunctionName()
	function, err := l.getLambdaFunction(name)
	if err == nil {
		return function.Configuration.FunctionArn
	}
	fileName := filepath.Base(zipPath)
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.CreateFunction(context.TODO(), &lambda.CreateFunctionInput{
		Code: &lambdaTypes.FunctionCode{
			S3Bucket: aws.String(l.config.GetBucket()),
			S3Key:    aws.String(fileName),
		},
		FunctionName: aws.String(l.config.GetFunctionName()),
		Description:  aws.String(l.description),
		Role:         l.roleARN,
		Runtime:      lambdaTypes.Runtime(l.config.GetRuntime()),
		Handler:      aws.String(l.functionHandler),
		Timeout:      aws.Int32(l.timeout),
	})
	if err != nil {
		utils.BulabaException(err)
	}
	return resp.FunctionArn
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

func (l *Lambda) updateLambdaFunction(content []byte) *string {
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.UpdateFunctionCode(context.TODO(), &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
		ZipFile:      content,
		Publish:      true,
	})
	if err != nil {
		utils.BulabaException(err)
	}
	return resp.FunctionArn
}

func (l *Lambda) ensureIAMRolePolicy() {
	fmt.Println("Fetching IAM Role Policy...")
	client := iam.NewFromConfig(l.AwsConfig)
	_, err := client.GetRolePolicy(context.TODO(), &iam.GetRolePolicyInput{
		RoleName:   &l.roleName,
		PolicyName: &l.policyName,
	})
	if err != nil {
		var nseErr *iamTypes.NoSuchEntityException
		if errors.As(err, &nseErr) {
			fmt.Println("IAM Role Policy not found. Creating new IAM Role Policy...")
			_, perr := client.PutRolePolicy(context.TODO(), &iam.PutRolePolicyInput{
				RoleName:       &l.roleName,
				PolicyName:     &l.policyName,
				PolicyDocument: aws.String(awsAttachPolicy),
			})
			if perr != nil {
				utils.BulabaException(perr.Error())
			}
			return
		}
		utils.BulabaException(err.Error())
	}
}

func (l *Lambda) getIAMRole() *iamTypes.Role {
	fmt.Println("Fetching IAM Role...")
	client := iam.NewFromConfig(l.AwsConfig)
	resp, err := client.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: &l.roleName,
	})
	if err != nil {
		var nseErr *iamTypes.NoSuchEntityException
		if errors.As(err, &nseErr) {
			fmt.Println("IAM Role not found. Creating new IAM Role ...")
			resp := l.createIAMRole()
			return resp.Role
		}
		utils.BulabaException(err.Error())
		return nil
	}

	return resp.Role
}

func (l *Lambda) createIAMRole() *iam.CreateRoleOutput {
	client := iam.NewFromConfig(l.AwsConfig)
	resp, err := client.CreateRole(context.TODO(), &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(awsAssumePolicy),
		Path:                     aws.String("/"),
		RoleName:                 &l.roleName,
	})
	if err != nil {
		utils.BulabaException(err.Error())
	}

	fmt.Printf("Created Role: %s\n", *resp.Role.RoleName)
	return resp
}
