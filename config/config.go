package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spatocode/jerm/internal/utils"
)

const (
	Dev               Stage = "dev"
	Production        Stage = "production"
	Staging           Stage = "staging"
)

type Stage string

type Config struct {
	Name    string  `json:"name"`
	Stage   string  `json:"stage"`
	Bucket  string  `json:"s3_bucket"`
	Region  string  `json:"region"`
	Lambda  *Lambda `json:"lambda"`
	Dir     string	`json:"dir"`
	Entry   string	`json:"entry"`
}

func (c *Config) Defaults() error {
	if err := c.detectRegion(); err != nil {
		return err
	}

	return nil
}

func (c *Config) detectRegion() error {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	if r := cfg.Region; r != "" {
		slog.Debug("extract region %s from aws default config", r)
		c.Region = cfg.Region
		return nil
	}

	r := "us-west-2"
	slog.Debug("default region %s", r)
	c.Region = r
	return nil
}

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

func (c *Config) init() error {
	workDir, err := os.Getwd()
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	splitPath := strings.Split(workDir, "/")
	projectName := splitPath[len(splitPath)-1]

	c.Stage = string(Dev)
	c.Bucket = fmt.Sprintf("jerm-%d", time.Now().Unix())
	c.Name = fmt.Sprintf("%s-%s", projectName, string(Dev))
	c.Dir = workDir

	if err := c.Defaults(); err != nil {
		utils.LogWarn(err.Error())
	}

	return nil
}

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

func ParseConfig(data []byte) (*Config, error) {
	// TODO: validate the configuration
	config := &Config{}
	err := json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
