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

const (
	DefaultTimeout      = 30
	DefaultWaitDuration = 20
	DefaultMaxRetry     = 3
)

// Lambda is the AWS Lambda operations
type Lambda struct {
	access            *IAM
	storage           jerm.CloudStorage
	logs              jerm.CloudMonitor
	apigateway        *ApiGateway
	functionHandler   string
	description       string
	awsConfig         aws.Config
	config            *config.Config
	retry             int
	maxWaiterDuration time.Duration
	timeout           int32
}

// NewLambda instantiates a new AWS Lambda service
func NewLambda(cfg *config.Config) (*Lambda, error) {
	l := &Lambda{
		description:       "Jerm Deployment",
		config:            cfg,
		retry:             DefaultMaxRetry,
		maxWaiterDuration: DefaultWaitDuration,
		timeout:           DefaultTimeout,
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

	l.logs = NewCloudWatch(cfg, *awsConfig)
	l.storage = NewS3(cfg, *awsConfig)
	l.access = NewIAM(cfg, *awsConfig)
	l.apigateway = NewApiGateway(cfg, *awsConfig)

	go func() {
		err := l.config.ToJson(jerm.DefaultConfigFile)
		if err != nil {
			log.PrintWarn(err)
		}
	}()

	err = l.access.checkPermissions()
	if err != nil {
		return nil, err
	}

	return l, nil
}

// Build builds the deployment package for lambda
func (l *Lambda) Build() (string, error) {
	log.Debug("building Jerm project for Lambda...")

	r := config.NewRuntime()
	if l.config.Entry == "" {
		l.config.Entry = r.Entry()
	}

	go func() {
		err := l.config.ToJson(jerm.DefaultConfigFile)
		if err != nil {
			log.PrintWarn(err)
		}
	}()

	handler, err := r.Build(l.config)
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

// Logs shows AWS logs
func (l *Lambda) Logs() {
	l.logs.Monitor()
}

func (l *Lambda) Deploy(zipPath string) (bool, error) {
	deployed, err := l.isAlreadyDeployed()
	if err != nil {
		return false, err
	}

	if deployed {
		return true, nil
	}

	l.storage.Upload(zipPath)
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

	err = l.storage.Delete(zipPath)
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
	err := l.storage.Upload(zipPath)
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
	l.scheduleEvents()

	err = l.apigateway.setup(functionArn)
	if err != nil {
		return err
	}

	err = utils.RemoveLocalFile(zipPath)
	if err != nil {
		return err
	}

	err = l.storage.Delete(zipPath)
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
	l.logs.DeleteLog()

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

// Rollback rolls back a Lambda deployment to a number of previous versions `revision`
func (l *Lambda) Rollback(steps int) error {
	var revisions []int
	versions, err := l.listLambdaVersions()
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			msg := "can't find a deployed project. Run 'jerm deploy' to deploy instead"
			return errors.New(msg)
		}
		return err
	}
	if len(versions) > 1 && versions[len(versions)-1].PackageType == "Image" {
		msg := "rollback unavailable for Docker deployment. Aborting"
		return errors.New(msg)
	}

	if len(versions) <= steps+1 {
		msg := "invalid revision for rollback. Aborting"
		return errors.New(msg)
	}

	for _, revision := range versions {
		version, _ := strconv.Atoi(*revision.Version)
		revisions = append(revisions, version)
	}

	sort.Slice(revisions, func(i int, j int) bool {
		return revisions[i] > revisions[j]
	})

	name := fmt.Sprintf("%s:%v", l.config.Name, revisions[steps])
	function, err := l.getLambdaFunction(name)
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			msg := fmt.Sprintf("can't find a lambda function %s", name)
			return errors.New(msg)
		}
		return err
	}

	location := function.Code.Location
	log.Debug("fetching function code location...")
	res, err := utils.Request(*location)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		msg := fmt.Sprintf("Unable to get version %v of project %s", revisions[steps], l.config.Name)
		return errors.New(msg)
	}
	defer res.Body.Close()

	log.Debug("reading fetched data...")
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	l.updateLambdaFunction(b)
	return nil
}

func (l *Lambda) listLambdaVersions() ([]lambdaTypes.FunctionConfiguration, error) {
	log.Debug("list lambda versions by function...")
	client := lambda.NewFromConfig(l.awsConfig)
	response, err := client.ListVersionsByFunction(context.TODO(), &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(l.config.Name),
	})
	if err != nil {
		return nil, err
	}
	return response.Versions, err
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
	log.Debug("fetching function code location...")
	versions, err := l.listLambdaVersions()
	if err != nil {
		var rnfErr *lambdaTypes.ResourceNotFoundException
		if errors.As(err, &rnfErr) {
			return false, nil
		}
		return false, err
	}
	return len(versions) > 0, nil
}

func (l *Lambda) createLambdaFunction(zipPath string) (*string, error) {
	name := l.config.Name
	function, err := l.getLambdaFunction(name)
	if err == nil {
		return function.Configuration.FunctionArn, nil
	}
	fileName := filepath.Base(zipPath)
	client := lambda.NewFromConfig(l.awsConfig)
	log.Debug("creating lambda function...")
	resp, err := client.CreateFunction(context.TODO(), &lambda.CreateFunctionInput{
		Code: &lambdaTypes.FunctionCode{
			S3Bucket: aws.String(l.config.Bucket),
			S3Key:    aws.String(fileName),
		},
		FunctionName: aws.String(name),
		Description:  aws.String(l.description),
		Role:         &l.config.Lambda.Role,
		Runtime:      lambdaTypes.Runtime(l.config.Lambda.Runtime),
		Handler:      aws.String(l.functionHandler),
		Timeout:      aws.Int32(l.timeout),
		Publish:      true,
	})
	if err != nil {
		return nil, err
	}
	return resp.FunctionArn, nil
}

func (l *Lambda) getLambdaFunction(name string) (*lambda.GetFunctionOutput, error) {
	log.Debug(fmt.Sprintf("getting lambda functon %s...", name))
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
	log.Debug("updating lambda function code...")
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
