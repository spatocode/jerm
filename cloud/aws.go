package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/spatocode/bulaba/utils"
)

type Lambda struct {
	roleARN         *string
	roleName        string
	policyName      string
	functionHandler string
	AwsConfig       aws.Config
	AwsCreds        aws.Credentials
	config          CloudConfig
}

func LoadLambda(cfg CloudConfig) *Lambda {
	l := &Lambda{
		roleName:   fmt.Sprintf("%s-BulabaLambdaServiceExecutionRole", cfg.GetFunctionName()),
		policyName: "bulaba-permissions",
		config:     cfg,
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
	startTime := int64(time.Millisecond*100000)
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
		response *cloudwatchlogs.FilterLogEventsOutput
		logEvents []cwTypes.FilteredLogEvent
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
		LogGroupName: aws.String(logName),
		LogStreamNames: streamNames,
		StartTime: aws.Int64(startTime),
		EndTime: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
		Limit: aws.Int32(10000),
		Interleaved: aws.Bool(true),
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
		Descending: aws.Bool(true),
		OrderBy: cwTypes.OrderByLastEventTime,
	})
	if err != nil {
		return nil, err
	}
	return resp.LogStreams, err
}

func (l *Lambda) Deploy(zipPath string) {
	l.ensureIsNotAlreadyDeployed()
	l.uploadFileToS3(zipPath)
	l.createLambdaFunction(zipPath)
	l.createAPIGateway()
}

func (l *Lambda) Rollback() {
	fmt.Println("Rolling back deployment...")
	var (
		revisions []*string
	)
	steps := 1
	response, err := l.listLambdaVersions()
	if err != nil {
		utils.BulabaException(err)
	}
	if len(response.Versions) > 1 && response.Versions[len(response.Versions)-1].PackageType == "Image" {
		utils.BulabaException("Rollback unavailable for Docker deployment. Aborting...")
	}

	if len(response.Versions) < steps + 1{
		utils.BulabaException("Invalid revision for rollback. Aborting...")
	}

	for _, revision := range response.Versions {
		if *revision.Version != "$LATEST" {
			revisions = append(revisions, revision.Version)
		}
	}
	sort.Slice(revisions, func(i int, j int) bool {
		return *revisions[i] > *revisions[j]
	})

	name := fmt.Sprintf("function%s:%s", l.config.GetFunctionName(), *revisions[steps])
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

	_, err = l.updateLambdaFunction(b)
	if err != nil {
		utils.BulabaException(err)
	}
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

	_, err = f.Write([]byte(awsLambdaHandler))
	if err != nil {
		utils.BulabaException(err.Error())
	}
	l.functionHandler = "handler.lambda_handler"
}

func (l *Lambda) createAPIGateway() {
	apiGatewayClient := apigatewayv2.NewFromConfig(l.AwsConfig)
	apiID, err := l.createAPI(apiGatewayClient)
	if err != nil {
		utils.BulabaException(err)
	}

	lambdaClient := lambda.NewFromConfig(l.AwsConfig)
	if err := l.createLambdaIntegration(apiGatewayClient, lambdaClient, apiID); err != nil {
		utils.BulabaException(err)
	}

	if err := l.createStage(apiGatewayClient, apiID); err != nil {
		utils.BulabaException(err)
	}

	fmt.Printf("API Gateway URL: https://%s.execute-api.%s.amazonaws.com/%s\n", *apiID, l.AwsConfig.Region, l.config.GetStage())
}

func (l *Lambda) createAPI(apiGatewayClient *apigatewayv2.Client) (*string, error) {
	resp, err := apiGatewayClient.CreateApi(context.TODO(), &apigatewayv2.CreateApiInput{
		Name:         aws.String(l.config.GetFunctionName()),
		ProtocolType: types.ProtocolType("HTTP"),
	})
	if err != nil {
		return nil, err
	}
	return resp.ApiId, nil
}

func (l *Lambda) getAccountID() string {
	stsClient := sts.NewFromConfig(l.AwsConfig)
	resp, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		utils.BulabaException(err)
	}
	return *resp.Account
}

func (l *Lambda) createLambdaIntegration(apiGatewayClient *apigatewayv2.Client, lambdaClient *lambda.Client, apiID *string) error {
	lambdaARN := fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", l.AwsConfig.Region, l.getAccountID(), l.config.GetFunctionName())
	_, err := apiGatewayClient.CreateIntegration(context.TODO(), &apigatewayv2.CreateIntegrationInput{
		ApiId:                apiID,
		IntegrationType:      types.IntegrationTypeAwsProxy,
		IntegrationUri:       &lambdaARN,
		IntegrationMethod:    aws.String("POST"),
		PayloadFormatVersion: aws.String("1.0"),
	})
	return err
}

func (l *Lambda) createStage(apiGatewayClient *apigatewayv2.Client, apiID *string) error {
	_, err := apiGatewayClient.CreateStage(context.TODO(), &apigatewayv2.CreateStageInput{
		ApiId:     apiID,
		StageName: aws.String(l.config.GetStage()),
	})
	return err
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
		utils.BulabaException("Encountered error while uploading package.")
	}
}

func (l *Lambda) ensureIsNotAlreadyDeployed() {
	versions := l.getLambdaVersionsByFuction()
	if len(versions) > 0 {
		msg := "Project already deployed. Run 'bulaba update' to update instead"
		utils.BulabaException(msg)
	}
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
		Role:         l.roleARN,
		Runtime:      lambdaTypes.Runtime(l.config.GetRuntime()),
		Handler:      aws.String(l.functionHandler),
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

func (l *Lambda) updateLambdaFunction(content []byte) (*lambda.UpdateFunctionCodeOutput, error) {
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.UpdateFunctionCode(context.TODO(), &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
		ZipFile: content,
		Publish: true,
	})
	if err != nil {
		return nil, err
	}
	return resp, err
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
