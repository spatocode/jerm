package cloud

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/spatocode/bulaba/utils"
)

type Lambda struct {
	name          string
	roleARN       *string
	roleName      string
	policyName    string
	AwsConfig     aws.Config
}

func NewLambda() *Lambda {
	l := &Lambda{
		roleName:   "BulabaLambdaExecutionRole",
		policyName: "bulaba-permissions",
	}
	l.AwsConfig = l.getAWSConfig()
	return l
}

func (l *Lambda) CheckPermissions() {
	l.roleARN = l.getIAMRole().Arn
	l.ensureIAMRolePolicy()
}

func (l *Lambda) getAWSConfig() aws.Config {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		utils.BulabaException(err.Error())
	}
	return cfg
}

func (l *Lambda) Deploy() {
	l.ensureIsNotAlreadyDeployed()
}

func (l *Lambda) CreateFunction(file string) {
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
		FunctionName: &l.name,
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
			fmt.Printf("IAM Role not found. Creating new IAM Role %s...\n", l.roleName)
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

	fmt.Printf("Created Role: %s", *resp.Role.RoleName)
	return resp
}
