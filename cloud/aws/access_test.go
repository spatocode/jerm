package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/aws/smithy-go/middleware"
	"github.com/spatocode/jerm/config"
	"github.com/stretchr/testify/assert"
)

func TestNewIAM(t *testing.T) {
	assert := assert.New(t)
	cfg := &config.Config{}
	awsC := aws.Config{}
	i := NewIAM(cfg, awsC)
	assert.Equal(cfg, i.config)
	assert.NotNil(i.client)
}

func TestCreateRole(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "create role failure",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"CreateRoleErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("CreateRoleError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error IAM: CreateRole, CreateRoleError"),
			wantErr: true,
		},
		{
			name: "create role successfull",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"CreateRoleMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &iam.CreateRoleOutput{},
								}, middleware.Metadata{}, nil
							},
						),
						middleware.Before,
					)
				},
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			awsCfg, err := awsConfig.LoadDefaultConfig(
				context.TODO(),
				awsConfig.WithRegion("us-west-1"),
				awsConfig.WithAPIOptions([]func(*middleware.Stack) error{tt.args.withAPIOptionsFunc}),
			)
			if err != nil {
				t.Fatal(err)
			}

			cfg := &config.Config{}
			iamClient := NewIAM(cfg, awsCfg)
			_, err = iamClient.createIAMRole()
			if (err != nil) != tt.wantErr {
				assert.Errorf(err, "error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.want.Error() {
				assert.EqualError(err, tt.want.Error())
			}
		})
	}
}

func TestGetIAMRole(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "get IAM role failure",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"GetRoleErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("GetRoleError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error IAM: GetRole, GetRoleError"),
			wantErr: true,
		},
		{
			name: "get IAM role successfull",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"GetRoleMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &iam.GetRoleOutput{},
								}, middleware.Metadata{}, nil
							},
						),
						middleware.Before,
					)
				},
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			awsCfg, err := awsConfig.LoadDefaultConfig(
				context.TODO(),
				awsConfig.WithRegion("us-west-1"),
				awsConfig.WithAPIOptions([]func(*middleware.Stack) error{tt.args.withAPIOptionsFunc}),
			)
			if err != nil {
				t.Fatal(err)
			}

			cfg := &config.Config{}
			iamClient := NewIAM(cfg, awsCfg)
			_, err = iamClient.getIAMRole()
			if (err != nil) != tt.wantErr {
				assert.Errorf(err, "error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.want.Error() {
				assert.EqualError(err, tt.want.Error())
			}
		})
	}
}

func TestEnsureIAMRolePolicy(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "Get IAM role policy failure",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"GetRolePolicyErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("GetRolePolicyError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error IAM: GetRolePolicy, GetRolePolicyError"),
			wantErr: true,
		},
		{
			name: "Get IAM role policy successfull",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"GetRolePolicyMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &iam.GetRolePolicyOutput{},
								}, middleware.Metadata{}, nil
							},
						),
						middleware.Before,
					)
				},
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			awsCfg, err := awsConfig.LoadDefaultConfig(
				context.TODO(),
				awsConfig.WithRegion("us-west-1"),
				awsConfig.WithAPIOptions([]func(*middleware.Stack) error{tt.args.withAPIOptionsFunc}),
			)
			if err != nil {
				t.Fatal(err)
			}

			cfg := &config.Config{}
			iamClient := NewIAM(cfg, awsCfg)
			err = iamClient.ensureIAMRolePolicy()
			if (err != nil) != tt.wantErr {
				assert.Errorf(err, "error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.want.Error() {
				assert.EqualError(err, tt.want.Error())
			}
		})
	}
}
