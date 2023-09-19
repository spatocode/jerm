package config

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

const (
	Dev            Stage = "dev"
	Production     Stage = "production"
	Staging        Stage = "staging"
	DefaultRegion        = "us-west-2"
	DefaultStage   Stage = Dev
	jermIgnoreFile       = ".jermignore"
)

type Stage string

// Config is the Jerm configuration details
type Config struct {
	Name   string  `json:"name"`
	Stage  string  `json:"stage"`
	Bucket string  `json:"bucket"`
	Region string  `json:"region"`
	Lambda *Lambda `json:"lambda"`
	Dir    string  `json:"dir"`
	Entry  string  `json:"entry"`
}

func (c *Config) GetFunctionName() string {
	return fmt.Sprintf("%s-%s", c.Name, c.Stage)
}

// extractRegion detects an AWS region from the AWS credentials
func (c *Config) extractRegion() error {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	if r := cfg.Region; r != "" {
		log.Debug(fmt.Sprintf("extract region %s from aws default config", r))
		c.Region = cfg.Region
		return nil
	}

	log.Debug(fmt.Sprintf("default region %s", DefaultRegion))
	c.Region = DefaultRegion
	return nil
}

// ToJson writes Config to json file
func (c *Config) ToJson(name string) error {
	b, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return err
	}

	f, err := os.Create(name)
	if err != nil {
		return err
	}

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return nil
}

// defaults to default configuration
func (c *Config) defaults() error {
	workDir, err := os.Getwd()
	if err != nil {
		log.Debug(err.Error())
		return err
	}

	workspace, err := utils.GetWorkspaceName()
	if err != nil {
		log.Debug(err.Error())
		return err
	}

	c.Stage = string(DefaultStage)
	c.Bucket = fmt.Sprintf("jerm-%d", time.Now().Unix())
	c.Name = workspace
	c.Dir = workDir

	if err = c.extractRegion(); err != nil {
		return err
	}

	return nil
}

func (c *Config) PromptConfig() (*Config, error) {
	c.defaults()

	var bucket string
	name, err := utils.ReadPromptInput(fmt.Sprintf("Project name [%s]:", c.Name), os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("unexpected error occured %s", err)
	}
	if name != "" {
		c.Name = name
	}

	stage, err := utils.ReadPromptInput(fmt.Sprintf("Deployment stage [%s]:", DefaultStage), os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("unexpected error occured %s", err)
	}
	if stage != "" {
		c.Stage = stage
	}

	region, err := utils.ReadPromptInput(fmt.Sprintf("Region [%s]:", c.Region), os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("unexpected error occured %s", err)
	}
	if region != "" {
		c.Region = region
	}

	for {
		bucket, err := utils.ReadPromptInput(fmt.Sprintf("Bucket [%s]:", fmt.Sprintf("jerm-%d", time.Now().Unix())), os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("unexpected error occured %s", err)
		}
		if bucket == "" || c.isValidAwsS3BucketName(bucket) {
			break
		}
		log.PrintWarn("Invalid bucket naming. Enter a valid bucket name.")
		log.PrintWarn("See here:", "https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html")
	}
	if bucket != "" {
		c.Bucket = strings.ToLower(bucket)
	}

	return c, nil
}

func (c *Config) isValidAwsS3BucketName(name string) bool {
	pattern := "^[a-z0-9.-]+$"
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return (len(name) >= 3 && len(name) <= 63) && regex.MatchString(name) &&
		!strings.Contains(name, "..") &&
		!strings.HasPrefix(name, "xn--") &&
		!strings.HasPrefix(name, "sthree") &&
		!strings.HasPrefix(name, "sthree-configurator") &&
		!strings.HasSuffix(name, "-s3alias") &&
		!strings.HasSuffix(name, "--ol-s3") &&
		net.ParseIP(name) == nil
}

func ReadIgnoredFiles(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileScanner := bufio.NewScanner(f)
	fileScanner.Split(bufio.ScanLines)
	var fileLines []string

	for fileScanner.Scan() {
		if strings.TrimSpace(fileScanner.Text()) == "" {
			continue
		}
		fileLines = append(fileLines, strings.TrimSpace(fileScanner.Text()))
	}

	return fileLines, nil
}

// ReadConfig reads a configuration file
func ReadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfig(data)
}

// ParseConfig parses a configuration data to Config struct
func ParseConfig(data []byte) (*Config, error) {
	// TODO: validate the configuration
	config := &Config{}
	err := json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
