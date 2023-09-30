package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/fatih/color"

	"github.com/aws/smithy-go/middleware"
	"github.com/spatocode/jerm/config"
	"github.com/stretchr/testify/assert"
)

func TestNewCloudWatch(t *testing.T) {
	assert := assert.New(t)
	cfg := &config.Config{}
	awsC := aws.Config{}
	s := NewCloudWatch(cfg, awsC)
	assert.Equal(cfg, s.config)
	assert.NotNil(s.client)
}

func TestCloudWatchClear(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "delete log group failure",
			args: args{
				name: "groupName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"DeleteLogGroupErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("DeleteLogGroupError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error CloudWatch Logs: DeleteLogGroup, DeleteLogGroupError"),
			wantErr: true,
		},
		{
			name: "delete log group successfull",
			args: args{
				name: "groupName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"DeleteLogGroupMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &cloudwatchlogs.DeleteLogGroupOutput{},
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
			client := NewCloudWatch(cfg, awsCfg)
			err = client.Clear()
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

func TestCloudWatchGetLogStreams(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "get log stream failure",
			args: args{
				name: "logName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"DescribeLogStreamsErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("DescribeLogStreamsError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error CloudWatch Logs: DescribeLogStreams, DescribeLogStreamsError"),
			wantErr: true,
		},
		{
			name: "describe log stream successfull",
			args: args{
				name: "groupName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"DescribeLogStreamsMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &cloudwatchlogs.DescribeLogStreamsOutput{},
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
			client := NewCloudWatch(cfg, awsCfg)
			out, err := client.getLogStreams(tt.args.name)
			if (err != nil) != tt.wantErr {
				assert.Errorf(err, "error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.want.Error() {
				assert.EqualError(err, tt.want.Error())
			}
			assert.IsType(out, []cwTypes.LogStream{})
		})
	}
}

func TestCloudWatchCreateLogStreams(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "create log stream failure",
			args: args{
				name: "logName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"CreateLogGroupErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("CreateLogGroupError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error CloudWatch Logs: CreateLogGroup, CreateLogGroupError"),
			wantErr: true,
		},
		{
			name: "create log stream successfull",
			args: args{
				name: "logName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"CreateLogGroupMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &cloudwatchlogs.CreateLogGroupOutput{},
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
			client := NewCloudWatch(cfg, awsCfg)
			err = client.createLogStreams(tt.args.name)
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

func TestCloudWatchFilterLogEvents(t *testing.T) {
	assert := assert.New(t)

	cases := []tcase{
		{
			name: "filter log events failure",
			args: args{
				name: "logName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"FilterLogEventsErrorMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: nil,
								}, middleware.Metadata{}, fmt.Errorf("FilterLogEventsError")
							},
						),
						middleware.Before,
					)
				},
			},
			want:    fmt.Errorf("operation error CloudWatch Logs: FilterLogEvents, FilterLogEventsError"),
			wantErr: true,
		},
		{
			name: "filter log events successfull",
			args: args{
				name: "logName",
				withAPIOptionsFunc: func(s *middleware.Stack) error {
					return s.Finalize.Add(
						middleware.FinalizeMiddlewareFunc(
							"FilterLogEventsMock",
							func(ctx context.Context, fi middleware.FinalizeInput, fh middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
								return middleware.FinalizeOutput{
									Result: &cloudwatchlogs.FilterLogEventsOutput{},
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
			client := NewCloudWatch(cfg, awsCfg)
			out, err := client.filterLogEvents(tt.args.name, []string{""}, 12, &cloudwatchlogs.FilterLogEventsOutput{})
			if (err != nil) != tt.wantErr {
				assert.Errorf(err, "error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.want.Error() {
				assert.EqualError(err, tt.want.Error())
			}
			assert.IsType(out, &cloudwatchlogs.FilterLogEventsOutput{})
		})
	}
}

func TestCloudWatchPrintLogs(t *testing.T) {
	assert := assert.New(t)

	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	color.Output = w

	awsCfg, err := awsConfig.LoadDefaultConfig(
		context.TODO(),
		awsConfig.WithRegion("us-west-1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	events := []cwTypes.FilteredLogEvent{
		{
			Timestamp: aws.Int64(40),
			Message:   aws.String("testevent1"),
		},
		{
			Timestamp: aws.Int64(40),
			Message:   aws.String("testevent2"),
		},
		{
			Timestamp: aws.Int64(40),
			Message:   aws.String("testevent REPORT RequestId"),
		},
		{
			Timestamp: aws.Int64(40),
			Message:   aws.String("testevent START RequestId"),
		},
		{
			Timestamp: aws.Int64(40),
			Message:   aws.String("testevent END RequestId"),
		},
	}
	client := NewCloudWatch(cfg, awsCfg)
	client.printLogs(events)

	w.Close()
	out, _ := io.ReadAll(r)
	color.Output = stdout

	expected := "[1970-01-01 01:00:00 +0100 WAT] testevent1\n[1970-01-01 01:00:00 +0100 WAT] testevent2\n"
	assert.Equal(expected, string(out))
}
