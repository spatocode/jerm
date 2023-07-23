package cloud

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"

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

func (l *Lambda) Deploy(zipPath string) {
	l.ensureIsNotAlreadyDeployed()
	l.uploadFileToS3(zipPath)
	l.createLambdaFunction(zipPath)
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
	l.functionHandler = file
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
	arn, err := l.getLambdaFunctionArn()
	if err == nil {
		return arn
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

func (l *Lambda) getLambdaFunctionArn() (*string, error) {
	client := lambda.NewFromConfig(l.AwsConfig)
	resp, err := client.GetFunction(context.TODO(), &lambda.GetFunctionInput{
		FunctionName: aws.String(l.config.GetFunctionName()),
	})
	if err != nil {
		return nil, err
	}
	return resp.Configuration.FunctionArn, err
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
