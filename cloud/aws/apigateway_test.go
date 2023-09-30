package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/smithy-go/middleware"
	"github.com/spatocode/jerm/config"
	"github.com/stretchr/testify/assert"
)

func TestNewApiGateway(t *testing.T) {
	assert := assert.New(t)
	cfg := &config.Config{}
	awsC := aws.Config{}
	a := NewApiGateway(cfg, awsC)
	assert.Equal(cfg, a.config)
	assert.NotNil(a.client)
}

func TestNewApiGatewayWithMonitor(t *testing.T) {
	assert := assert.New(t)
	cfg := &config.Config{}
	awsC := aws.Config{}
	a := NewApiGateway(cfg, awsC)
	monitor := NewCloudWatch(cfg, awsC)
	a.WithMonitor(monitor)
	assert.Equal(monitor, a.monitor)
}

func TestApiGatewayGetRestApis(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "get rest apis failure",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"GetRestApisErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("GetRestApisError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error API Gateway: GetRestApis, GetRestApisError"),
			wantErr: true,
		},
		{
			name: "get rest apis successfull",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"GetRestApisMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &apigateway.GetRestApisOutput{},
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
			client := NewApiGateway(cfg, awsCfg)
			apis, err := client.getRestApis()
			if (err != nil) != tt.wantErr {
				assert.Errorf(err, "error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.want.Error() {
				assert.EqualError(err, tt.want.Error())
			}
			assert.IsType(apis, []*string{})
		})
	}
}
