package config

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spatocode/jerm/internal/log"
)

const (
	Dev            Stage = "dev"
	Production     Stage = "production"
	Staging        Stage = "staging"
	DefaultRegion        = "us-west-2"
	jermIgnoreFile       = ".jermignore"
)

type Stage string

// Config is the Jerm configuration details
type Config struct {
	Name   string   `json:"name"`
	Stage  string   `json:"stage"`
	Bucket string   `json:"bucket"`
	Region string   `json:"region"`
	Lambda *Lambda  `json:"lambda"`
	Ignore []string `json:"ignore"`
	Dir    string   `json:"dir"`
	Entry  string   `json:"entry"`
}

// Defaults extracts the default configuration
func (c *Config) Defaults() error {
	if err := c.detectRegion(); err != nil {
		return err
	}

	return nil
}

// detectRegion detects an AWS region from the AWS credentials
func (c *Config) detectRegion() error {
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

// init initialize a configuration with it's default
func (c *Config) init() error {
	workDir, err := os.Getwd()
	if err != nil {
		log.Debug(err.Error())
		return err
	}
	splitPath := strings.Split(workDir, "/")
	projectName := splitPath[len(splitPath)-1]

	c.Stage = string(Dev)
	c.Bucket = fmt.Sprintf("jerm-%d", time.Now().Unix())
	c.Name = fmt.Sprintf("%s-%s", projectName, string(Dev))
	c.Dir = workDir

	if err := c.Defaults(); err != nil {
		log.PrintWarn(err.Error())
	}

	return nil
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
		cfg := &Config{}
		err := cfg.init()
		if err != nil {
			return nil, err
		}

		return cfg, nil
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
