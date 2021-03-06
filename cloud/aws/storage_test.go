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

type args struct {
	name               string
	withAPIOptionsFunc func(*middleware.Stack) error
}

type tcase struct {
	name    string
	args    args
	want    error
	wantErr bool
}

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

	cases := []tcase{
		{
			name: "delete object failure",
			args: args{
				name: "testobj",
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
		{
			name: "delete object successfully",
			args: args{
				name: "tstobject",
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

			cfg := &config.Config{Bucket: "testbucket"}
			s3Client := NewS3(cfg, awsCfg)
			err = s3Client.Delete(tt.args.name)
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

func TestS3HeadBucket(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "head bucket failure",
			args: args{
				name: "testobj",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"HeadBucketErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("HeadBucketError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error S3: HeadBucket, HeadBucketError"),
			wantErr: true,
		},
		{
			name: "head bucket success",
			args: args{
				name: "tstobject",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"HeadBucketMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &s3.HeadBucketOutput{},
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

			cfg := &config.Config{Bucket: "testbucket"}
			s3Client := NewS3(cfg, awsCfg)
			err = s3Client.headBucket()
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

func TestS3Upload(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "upload object failure",
			args: args{
				name: "../../assets/tests/testfile1",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"PutObjectErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("PutObjectError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error S3: PutObject, PutObjectError : encountered error while uploading package. Aborting"),
			wantErr: true,
		},
		{
			name: "upload object success",
			args: args{
				name: "../../assets/tests/testfile2",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"PutObjectMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &s3.PutObjectOutput{},
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

			cfg := &config.Config{Bucket: "testbucket"}
			s3Client := NewS3(cfg, awsCfg)
			err = s3Client.Upload(tt.args.name)
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

func TestS3CreateBucket(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "create bucket failure",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"CreateBucketErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("CreateBucketError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error S3: CreateBucket, CreateBucketError"),
			wantErr: true,
		},
		{
			name: "create bucket success",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"CreateBucketMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &s3.CreateBucketOutput{},
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

			cfg := &config.Config{Bucket: "testbucket"}
			s3Client := NewS3(cfg, awsCfg)
			err = s3Client.CreateBucket(true)
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

func TestS3Accessible(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "head bucket failure",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"HeadBucketErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("HeadBucketError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error S3: HeadBucket, HeadBucketError"),
			wantErr: true,
		},
		{
			name: "head bucket success",
			args: args{
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"HeadBucketMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &s3.HeadBucketOutput{},
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

			cfg := &config.Config{Bucket: "testbucket"}
			s3Client := NewS3(cfg, awsCfg)
			err = s3Client.Accessible()
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
