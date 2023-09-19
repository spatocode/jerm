package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	"github.com/spatocode/jerm/config"
	"github.com/stretchr/testify/assert"
)

func TestNewS3(t *testing.T) {
	assert := assert.New(t)
	cfg := &config.Config{}
	awsC := aws.Config{}
	s := NewS3(cfg, awsC)
	assert.Equal(cfg, s.config)
	assert.Equal(awsC, s.awsConfig)
	assert.NotNil(s.client)
}

func TestS3Delete(t *testing.T) {
	assert := assert.New(t)

	type args struct {
		objectName         string
		withAPIOptionsFunc func(*middleware.Stack) error
	}

	cases := []struct {
		name    string
		args    args
		want    error
		wantErr bool
	}{
		{
			name: "delete object successfully",
			args: args{
				objectName: "tstobject",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"DeleteObjectMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &s3.DeleteObjectOutput{},
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
		{
			name: "delete object failure",
			args: args{
				objectName: "testobj",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"DeleteObjectErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("DeleteObjectError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error S3: DeleteObject, DeleteObjectError"),
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
			if err != nil {
				t.Fatal(err)
			}

			cfg := &config.Config{}
			s3Client := NewS3(cfg, awsCfg)
			err = s3Client.Delete(tt.args.objectName)
			if tt.wantErr {
				assert.ErrorIs(err, tt.want)
			} else {
				assert.Nil(err)
			}
		})
	}

	// cfg := &config.Config{}
	// awsC := aws.Config{}
	// s := &S3{cfg, awsC, &s3.Client{}}
	// assert.Equal(cfg, s.config)
	// assert.Equal(awsC, s.awsConfig)
	// assert.NotNil(s.client)
}
