package create

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spatocode/bulaba/cloud"
	"github.com/spatocode/bulaba/config"
	"github.com/spatocode/bulaba/utils"
)

type Project struct {
	config *config.Config
}

func NewProject() *Project {
	// TODO: Detect project stack
	workDir, err := os.Getwd()
	if err != nil {
		utils.BulabaException(err.Error())
	}
	splitPath := strings.Split(workDir, "/")
	projectName := splitPath[len(splitPath)-1]
	config := &config.Config{
		Environment: config.Environment,
		S3Bucket:    config.S3Bucket,
		ProjectName: projectName,
	}
	project := &Project{
		config: config,
	}
	return project
}

func (p *Project) Init() {
	utils.EnsureProjectExists()
	p.printInitMessage()

	env := p.getStdIn(fmt.Sprintf("Deployment environment: (%s) [dev, staging, production]", config.Environment))
	if env != "\n" {
		fmt.Println(env != "\n", env != "")
		// TODO: Check is correct name
		p.config.Environment = env
	}

	awsConfig := cloud.NewLambda().AwsConfig
	p.config.AwsRegion = awsConfig.Region
	p.config.ProjectName = fmt.Sprintf("%s-%s", p.config.ProjectName, p.config.Environment)

	bucket := p.getStdIn(fmt.Sprintf("S3 Bucket: (%s)", config.S3Bucket))
	if bucket != "\n" {
		// TODO: Enforce bucket naming restrictions
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html#bucketnamingrules
		p.config.S3Bucket = bucket
	}

	p.writeConfigFile(p.config)
}

func (p *Project) writeConfigFile(c *config.Config) {
	b, err := json.Marshal(*c)
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

func (c *Project) printInitMessage() {
	fmt.Printf("This utility will walk you through configuring your bulaba deployment by creating a %s file.\n", "bulaba.json")
	fmt.Println()
}

func (c *Project) getStdIn(prompt string) string {
	fmt.Println(prompt)
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		utils.BulabaException(err.Error())
	}
	fmt.Println()
	return value
}
