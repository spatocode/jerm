package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
)

type IAM struct {
	awsConfig  aws.Config
	config     *config.Config
	roleName   string
	policyName string
}

// NewIAM creates a new AWS IAM object
func NewIAM(cfg *config.Config, awsConfig aws.Config) *IAM {
	return &IAM{
		config:     cfg,
		awsConfig:  awsConfig,
		roleName:   fmt.Sprintf("%s-JermLambdaServiceExecutionRole", cfg.Name),
		policyName: "jerm-permissions",
	}
}

// checkPermissions checks the neccessary permissions needed to access AWS account
func (i *IAM) checkPermissions() error {
	role, err := i.getIAMRole()
	if err != nil {
		return err
	}
	i.config.Lambda.Role = *role.Arn

	err = i.ensureIAMRolePolicy()
	if err != nil {
		return err
	}
	return nil
}

// ensureIAMRolePolicy ensures the required policy is available
// and creates/updates one if unavailable.
func (i *IAM) ensureIAMRolePolicy() error {
	log.Debug("fetching IAM role policy...")
	client := iam.NewFromConfig(i.awsConfig)
	_, err := client.GetRolePolicy(context.TODO(), &iam.GetRolePolicyInput{
		RoleName:   &i.roleName,
		PolicyName: &i.policyName,
	})
	if err != nil {
		var nseErr *iamTypes.NoSuchEntityException
		if errors.As(err, &nseErr) {
			log.Debug("IAM role policy not found. creating new IAM role policy...")
			_, perr := client.PutRolePolicy(context.TODO(), &iam.PutRolePolicyInput{
				RoleName:       &i.roleName,
				PolicyName:     &i.policyName,
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

// getIAMRole gets AWS IAM role
func (i *IAM) getIAMRole() (*iamTypes.Role, error) {
	log.Debug("fetching IAM role...")
	client := iam.NewFromConfig(i.awsConfig)
	resp, err := client.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: &i.roleName,
	})
	if err != nil {
		var nseErr *iamTypes.NoSuchEntityException
		if errors.As(err, &nseErr) {
			log.Debug("IAM role not found. creating new IAM role ...")
			resp, err := i.createIAMRole()
			return resp.Role, err
		}
		return nil, err
	}

	return resp.Role, nil
}

// createIAMRole creates AWS IAM role
func (i *IAM) createIAMRole() (*iam.CreateRoleOutput, error) {
	client := iam.NewFromConfig(i.awsConfig)
	resp, err := client.CreateRole(context.TODO(), &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(awsAssumePolicy),
		Path:                     aws.String("/"),
		RoleName:                 &i.roleName,
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}