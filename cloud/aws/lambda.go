package aws

import (
	"context"
	"encoding/base64"
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
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/spatocode/jerm"
	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

// Lambda is the AWS Lambda operations
type Lambda struct {
	iam               *IAM
	s3                jerm.CloudStorage
	cloudwatch        jerm.CloudMonitor
	apigateway        *ApiGateway
	functionHandler   string
	description       string
	awsConfig         aws.Config
	config            *config.Config
	defaultMaxRetry   int
	maxWaiterDuration time.Duration
	timeout           int32
}

// NewLambda instantiates a new AWS Lambda service
func NewLambda(cfg *config.Config) (*Lambda, error) {
	l := &Lambda{
		description:       "Jerm Deployment",
		config:            cfg,
		defaultMaxRetry:   3,
		maxWaiterDuration: 20,
		timeout:           30,
	}

	lambdaConfig := &config.Lambda{}
	err := lambdaConfig.Defaults()
	if err != nil {
		return nil, err
	}

	l.config.Lambda = lambdaConfig
	awsConfig, err := l.getAwsConfig()
	if err != nil {
		return nil, err
	}
	l.awsConfig = *awsConfig

	l.cloudwatch = NewCloudWatch(cfg, *awsConfig)
	l.s3 = NewS3(cfg, *awsConfig)
	l.iam = NewIAM(cfg, *awsConfig)

	err = l.config.ToJson(jerm.DefaultConfigFile)
	if err != nil {
		return nil, err
	}

	err = l.iam.checkPermissions()
	if err != nil {
		return nil, err
	}

	return l, nil
}

// Build builds the deployment package for lambda
func (l *Lambda) Build() (string, error) {
	log.Debug("building Jerm project for Lambda...")
	handler, err := config.NewPythonConfig().Build(l.config)
	dir := filepath.Dir(handler)
	if err != nil {
		return "", err
	}

	if l.config.Lambda.Handler == "" {
		err := l.CreateFunctionEntry(handler)
		return dir, err
	}
	return dir, nil
}

func (l *Lambda) Invoke(command string) error {
	payload := fmt.Sprintf(`{"manage": "%s"}`, command)
	return l.invokeLambdaFunction([]byte(payload))
}

// invokeLambdaFunction invokes a lambda function with payload
func (l *Lambda) invokeLambdaFunction(payload []byte) error {
	client := lambda.NewFromConfig(l.awsConfig)
	out, err := client.Invoke(context.TODO(), &lambda.InvokeInput{
		FunctionName:   &l.config.Name,
		InvocationType: lambdaTypes.InvocationTypeRequestResponse,
		LogType:        lambdaTypes.LogTypeTail,
		Payload:        payload,
	})
	if err != nil {
		return err
	}

	if out.LogResult != nil {
		rawText, err := base64.StdEncoding.DecodeString(*out.LogResult)
		if err != nil {
			return err
		}
		log.PrintInfo(string(rawText))
	} else {
		log.PrintInfo(*out)
	}

	if out.FunctionError != nil {
		return fmt.Errorf("%s - encountered an error while invoking function", *out.FunctionError)
	}

	return nil
}

// getAwsConfig fetches AWS account configuration
func (l *Lambda) getAwsConfig() (*aws.Config, error) {
	msg := fmt.Sprintf("Unable to find an AWS profile. Ensure you set up your AWS before using Jerm. See here for more info %s", awsConfigDocsUrl)
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, errors.New(msg)
	}
	return &cfg, nil
}

// Logs shows AWS Cloudwatch logs
func (l *Lambda) Logs() {
	l.cloudwatch.Monitor()
}

func (l *Lambda) Deploy(zipPath string) (bool, error) {
	deployed, err := l.isAlreadyDeployed()
	if err != nil {
		return false, err
	}

	if deployed {
		return true, nil
	}

	l.s3.Upload(zipPath)
	functionArn, err := l.createLambdaFunction(zipPath)
	if err != nil {
		return false, err
	}

	err = l.waitTillFunctionBecomesActive()
	if err != nil {
		return false, err
	}

	l.scheduleEvents()
	err = l.apigateway.setup(functionArn)
	if err != nil {
		return false, err
	}

	err = utils.RemoveLocalFile(zipPath)
	if err != nil {
		return false, err
	}

	err = l.s3.Delete(zipPath)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (l *Lambda) waitTillFunctionBecomesActive() error {
	client := lambda.NewFunctionActiveV2Waiter(lambda.NewFromConfig(l.awsConfig))
	err := client.Wait(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(l.config.Name),
	}, time.Second*l.maxWaiterDuration)
	if err != nil {
		return err
	}
	return nil
}

func (l *Lambda) waitTillFunctionBecomesUpdated() {
	client := lambda.NewFunctionUpdatedV2Waiter(lambda.NewFromConfig(l.awsConfig))
	err := client.Wait(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(l.config.Name),
	}, time.Second*l.maxWaiterDuration)
	if err != nil {
		log.Debug(err.Error())
	}
}

func (l *Lambda) scheduleEvents() {
	
}

func (l *Lambda) Update(zipPath string) error {
	err := l.s3.Upload(zipPath)
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

	l.waitTillFunctionBecomesUpdated()
	err = l.apigateway.setup(functionArn)
	if err != nil {
		return err
	}

	err = utils.RemoveLocalFile(zipPath)
	if err != nil {
		return err
	}

	err = l.s3.Delete(zipPath)
	if err != nil {
		return err
	}

	return nil
}

// Undeploy deletes a Lambda deployment
func (l *Lambda) Undeploy() error {
	deployed, err := l.isAlreadyDeployed()
	if err != nil {
		return err
	}
	if !deployed {
		msg := "can't find a deployed project. Run 'jerm deploy' to deploy instead"
		return errors.New(msg)
	}

	log.Debug("undeploying...")
	err = l.apigateway.delete()
	if err != nil {
		return err
	}

	err = l.apigateway.deleteLogs()
	if err != nil {
		return err
	}

	l.deleteLambdaFunction()
	l.cloudwatch.DeleteLog()

	return nil
}

// deleteLambdaFunction deletes a Lambda function
func (l *Lambda) deleteLambdaFunction() {
	log.Debug("deleting lambda function...")
	client := lambda.NewFromConfig(l.awsConfig)
	client.DeleteFunction(context.TODO(), &lambda.DeleteFunctionInput{
		FunctionName: aws.String(l.config.Name),
	})
}

// Rollback rolls back a Lambda deployment to the previous version
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
	log.PrintInfo("Done!")
	return nil
}

func (l *Lambda) listLambdaVersions() (*lambda.ListVersionsByFunctionOutput, error) {
	client := lambda.NewFromConfig(l.awsConfig)
	response, err := client.ListVersionsByFunction(context.TODO(), &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(l.config.Name),
	})
	return response, err
}

// CreateFunctionEntry creates a Lambda function handler file
func (l *Lambda) CreateFunctionEntry(file string) error {
	log.Debug("creating lambda handler...")
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

func (l *Lambda) isAlreadyDeployed() (bool, error) {
	versions, err := l.getLambdaVersionsByFuction()
	if err != nil {
		return false, err
	}
	return len(versions) > 0, nil
}

func (l *Lambda) getLambdaVersionsByFuction() ([]lambdaTypes.FunctionConfiguration, error) {
	client := lambda.NewFromConfig(l.awsConfig)
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
	client := lambda.NewFromConfig(l.awsConfig)
	resp, err := client.CreateFunction(context.TODO(), &lambda.CreateFunctionInput{
		Code: &lambdaTypes.FunctionCode{
			S3Bucket: aws.String(l.config.Bucket),
			S3Key:    aws.String(fileName),
		},
		FunctionName: aws.String(l.config.Name),
		Description:  aws.String(l.description),
		Role:         &l.config.Lambda.Role,
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
	client := lambda.NewFromConfig(l.awsConfig)
	resp, err := client.GetFunction(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(name),
	})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (l *Lambda) updateLambdaFunction(content []byte) (*string, error) {
	client := lambda.NewFromConfig(l.awsConfig)
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
