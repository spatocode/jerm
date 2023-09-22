package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	agTypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cf "github.com/awslabs/goformation/v7/cloudformation"
	cfApigateway "github.com/awslabs/goformation/v7/cloudformation/apigateway"

	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

type ApiGateway struct {
	s3         *S3
	cfTemplate *cf.Template
	awsConfig  aws.Config
	config     *config.Config
}

func NewApiGateway(config *config.Config, awsConfig aws.Config) *ApiGateway {
	s3 := NewS3(config, awsConfig)
	return &ApiGateway{s3, nil, awsConfig, config}
}

func (a *ApiGateway) setup(functionArn *string) error {
	template := cf.NewTemplate()
	template.Description = "Auto generated by Jerm"
	restApi := &cfApigateway.RestApi{
		Name:        aws.String(a.config.GetFunctionName()),
		Description: aws.String("Automatically created by Jerm"),
	}
	template.Resources["Api"] = restApi

	rootId := cf.GetAtt("Api", "RootResourceId")
	a.cfTemplate = template
	a.createMethods(functionArn, rootId, 0)

	resource := &cfApigateway.Resource{}
	resource.RestApiId = cf.Ref("Api")
	resource.ParentId = rootId
	resource.PathPart = "{proxy+}"
	a.cfTemplate.Resources["ResourceAnyPathSlashed"] = resource
	a.createMethods(functionArn, cf.Ref("ResourceAnyPathSlashed"), 1)

	a.createCFStack()

	apiId, err := a.getApiId()
	if err != nil {
		return err
	}

	apiUrl, err := a.deploy(apiId)
	if err != nil {
		return err
	}

	fmt.Printf("%s %s", log.Magenta("url:"), log.Green(apiUrl))

	return nil
}

// deployAPIGateway deploys an AWS API gateway
func (a *ApiGateway) deploy(apiId *string) (string, error) {
	log.Debug("deploying API Gateway...")
	apiGatewayClient := apigateway.NewFromConfig(a.awsConfig)
	_, err := apiGatewayClient.CreateDeployment(context.TODO(), &apigateway.CreateDeploymentInput{
		StageName:        aws.String(a.config.Stage),
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
		StageName: aws.String(a.config.Stage),
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

	return fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s\n", *apiId, a.awsConfig.Region, a.config.Stage), nil
}

func (a *ApiGateway) getRestApis() ([]*string, error) {
	var apiIds []*string
	apiGatewayClient := apigateway.NewFromConfig(a.awsConfig)
	resp, err := apiGatewayClient.GetRestApis(context.TODO(), &apigateway.GetRestApisInput{
		Limit: aws.Int32(500),
	})
	if err != nil {
		return nil, err
	}

	for _, item := range resp.Items {
		if *item.Name == a.config.GetFunctionName() {
			apiIds = append(apiIds, item.Id)
		}
	}
	return apiIds, err
}

func (a *ApiGateway) getApiId() (*string, error) {
	cloudformationClient := cloudformation.NewFromConfig(a.awsConfig)
	resp, err := cloudformationClient.DescribeStackResource(context.TODO(), &cloudformation.DescribeStackResourceInput{
		StackName:         aws.String(a.config.GetFunctionName()),
		LogicalResourceId: aws.String("Api"),
	})
	if err != nil {
		apiId, err := a.getRestApis()
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

func (a *ApiGateway) createCFStack() error {
	template := fmt.Sprintf("%s-template-%v.json", a.config.GetFunctionName(), time.Now().Unix())
	data, err := a.cfTemplate.JSON()
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
	a.s3.Upload(template)

	url := fmt.Sprintf("https://s3.amazonaws.com/%s/%s", a.config.Bucket, template)
	if a.awsConfig.Region == "us-gov-west-1" {
		url = fmt.Sprintf("https://s3-us-gov-west-1.amazonaws.com/%s/%s", a.config.Bucket, template)
	}

	client := cloudformation.NewFromConfig(a.awsConfig)
	_, err = client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(a.config.GetFunctionName()),
	})
	if err != nil {
		log.Debug("creating cloud formation stack...")
		tags := []cfTypes.Tag{
			{
				Key:   aws.String("JermProject"),
				Value: aws.String(a.config.GetFunctionName()),
			},
		}
		_, err := client.CreateStack(context.TODO(), &cloudformation.CreateStackInput{
			StackName:    aws.String(a.config.GetFunctionName()),
			TemplateURL:  aws.String(url),
			Tags:         tags,
			Capabilities: make([]cfTypes.Capability, 0),
		})
		if err != nil {
			return err
		}
	} else {
		client.UpdateStack(context.TODO(), &cloudformation.UpdateStackInput{
			StackName:    aws.String(a.config.GetFunctionName()),
			TemplateURL:  aws.String(url),
			Capabilities: make([]cfTypes.Capability, 0),
		})
	}

	for {
		time.Sleep(time.Second * 3)
		resp, _ := client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
			StackName: aws.String(a.config.GetFunctionName()),
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

	err = a.s3.Delete(template)
	if err != nil {
		return err
	}

	err = utils.RemoveLocalFile(template)
	if err != nil {
		return err
	}

	return nil
}

func (a *ApiGateway) deleteStack() error {
	client := cloudformation.NewFromConfig(a.awsConfig)
	resp, err := client.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(a.config.GetFunctionName()),
	})
	if err != nil {
		log.Debug(fmt.Sprintf("unable to find stack %s\n", a.config.GetFunctionName()))
		return err
	}
	tags := make(map[string]string)
	for _, tag := range resp.Stacks[0].Tags {
		tags[*tag.Key] = *tag.Value
	}
	if tags["JermProject"] == a.config.GetFunctionName() {
		log.Debug("deleting cloud formation stack...")
		_, err := client.DeleteStack(context.TODO(), &cloudformation.DeleteStackInput{
			StackName: aws.String(a.config.GetFunctionName()),
		})
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("JermProject not found")
	}
	return nil
}

func (a *ApiGateway) createMethods(functionArn *string, resourceId string, depth int) {
	pre := "aws-us-gov"
	if a.awsConfig.Region != "us-gov-west-1" {
		pre = "aws"
	}
	integrationUri := fmt.Sprintf("arn:%s:apigateway:%s:lambda:path/2015-03-31/functions/%s/invocations", pre, a.awsConfig.Region, *functionArn)
	methodName := "ANY"
	method := &cfApigateway.Method{}

	method.RestApiId = cf.Ref("Api")
	method.ResourceId = resourceId
	method.HttpMethod = methodName
	method.AuthorizationType = aws.String("NONE")
	method.ApiKeyRequired = aws.Bool(false)
	a.cfTemplate.Resources[fmt.Sprintf("%s%v", methodName, depth)] = method

	method.Integration = &cfApigateway.Method_Integration{
		CacheNamespace:        aws.String("none"),
		Credentials:           &a.config.Lambda.Role,
		IntegrationHttpMethod: aws.String("POST"),
		Type:                  aws.String("AWS_PROXY"),
		PassthroughBehavior:   aws.String("NEVER"),
		Uri:                   &integrationUri,
	}
}

// deleteLogs deletes API gateway logs
func (a *ApiGateway) deleteLogs() error {
	log.Debug("deleting API Gateway logs...")
	apiIds, err := a.getRestApis()
	if err != nil {
		return err
	}
	for _, id := range apiIds {
		client := apigateway.NewFromConfig(a.awsConfig)
		resp, err := client.GetStages(context.TODO(), &apigateway.GetStagesInput{
			RestApiId: id,
		})
		if err != nil {
			return err
		}

		cw := NewCloudWatch(a.config, a.awsConfig)
		for _, item := range resp.Item {
			groupName := fmt.Sprintf("API-Gateway-Execution-Logs_%s/%s", *id, *item.StageName)
			cw.deleteLogGroup(groupName)
		}
	}
	return nil
}

// deleteAPIGateway deletes an API gateway
func (a *ApiGateway) delete() error {
	err := a.deleteStack()
	if err == nil {
		return nil
	}

	apiIds, err := a.getRestApis()
	if err != nil {
		return err
	}

	log.Debug("deleting API Gateway...")
	for _, id := range apiIds {
		client := apigateway.NewFromConfig(a.awsConfig)
		_, err := client.DeleteRestApi(context.TODO(), &apigateway.DeleteRestApiInput{
			RestApiId: id,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
