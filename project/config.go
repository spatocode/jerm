package project

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spatocode/bulaba/utils"
)

var (
	Stage             = "dev"
	InitFilename      = "bulaba.json"
	archiveFileSuffix = fmt.Sprintf("-pkg%d.zip", utils.GenerateRandomNumber())
)

type Config struct {
	Stage         string `json:"stage"`
	Bucket        string `json:"s3_bucket"`
	ProjectName   string `json:"project_name"`
	Region        string `json:"region"`
	Profile       string `json:"profile"`
	PythonVersion string `json:"python_version"`
}

func (c *Config) GetRuntime() string {
	return c.PythonVersion
}

func (c *Config) GetFunctionName() string {
	return c.ProjectName
}

func (c *Config) GetBucket() string {
	return c.Bucket
}

func (c *Config) GetStage() string {
	return c.Stage
}

func (c *Config) ToJson() {
	b, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		utils.BulabaException(err.Error())
	}

	f, err := os.Create("bulaba.json")
	if err != nil {
		utils.BulabaException(err.Error())
	}

	_, err = f.Write(b)
	if err != nil {
		utils.BulabaException(err.Error())
	}
}
