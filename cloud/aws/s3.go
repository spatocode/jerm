package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
)

type S3 struct {
	config    *config.Config
	awsConfig aws.Config
}

// NewS3 creates a new AWS S3 object
func NewS3(config *config.Config, awsConfig aws.Config) *S3 {
	return &S3{
		config:    config,
		awsConfig: awsConfig,
	}
}

// upload a file to AWS S3 bucket
func (s *S3) Upload(filePath string) error {
	client := s3.NewFromConfig(s.awsConfig)
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(s.config.Bucket),
	})
	if err != nil {
		var nfErr *s3Types.NotFound
		if errors.As(err, &nfErr) {
			err := s.createBucket(client, true)
			if err != nil {
				log.Debug("error on creating s3 bucket with config", true)
				err := s.createBucket(client, false)
				if err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	f, err := os.Stat(filePath)
	if f.Size() == 0 || err != nil {
		msg := "encountered issue with packaged file"
		return errors.New(msg)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileName := filepath.Base(filePath)
	log.Debug(fmt.Sprintf("uploading file %s...\n", fileName))
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(fileName),
		Body:   file,
	})
	if err != nil {
		msg := "encountered error while uploading package. Aborting"
		return errors.New(msg)
	}
	return nil
}

// delete a file from AWS S3 bucket
func (s *S3) Delete(filePath string) error {
	client := s3.NewFromConfig(s.awsConfig)
	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(s.config.Bucket),
	})
	if err != nil {
		return err
	}

	_, err = client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(filePath),
	})
	if err != nil {
		return err
	}
	return nil
}

// createBucket creates an AWS S3 bucket
func (s *S3) createBucket(client *s3.Client, isConfig bool) error {
	log.Debug("creating s3 bucket with config", isConfig)
	if isConfig {
		_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(s.config.Bucket),
			CreateBucketConfiguration: &s3Types.CreateBucketConfiguration{
				LocationConstraint: s3Types.BucketLocationConstraint(s.awsConfig.Region),
			},
		})
		return err
	}
	_, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(s.config.Bucket),
	})
	return err
}
