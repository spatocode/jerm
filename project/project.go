package project

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	cp "github.com/otiai10/copy"

	"github.com/spatocode/bulaba/cloud"
	"github.com/spatocode/bulaba/utils"
)

var (
	excludedFiles = []string{
		"*.zip",
		"*.exe",
		"*.git",
		"*.DS_Store",
		"pip",
		"venv",
		"__pycache__/*",
		"*.hg",
		"*.Python",
		"setuputils*",
		"*.tar.gz",
		".git/*",
		"docutils*",
	}
)

type Project struct {
	config *Config
	cloud  cloud.Platform
}

func NewProject() *Project {
	// TODO: Detect project stack
	workDir, err := os.Getwd()
	if err != nil {
		utils.BulabaException(err.Error())
	}
	splitPath := strings.Split(workDir, "/")
	projectName := splitPath[len(splitPath)-1]
	config := &Config{
		Environment: Environment,
		S3Bucket:    S3Bucket,
		ProjectName: projectName,
	}
	p := &Project{
		config: config,
	}
	p.config.PythonVersion = p.getPythonVersion()
	return p
}

func (p *Project) Init() {
	utils.EnsureProjectExists()
	p.printInitMessage()

	env := p.getStdIn(fmt.Sprintf("Deployment environment: (%s) [dev, staging, production]", Environment))
	if env != "\n" {
		fmt.Println(env != "\n", env != "")
		// TODO: Check is correct name
		p.config.Environment = env
	}

	awsConfig := cloud.NewLambda().AwsConfig
	p.config.AwsRegion = awsConfig.Region
	p.config.ProjectName = fmt.Sprintf("%s-%s", p.config.ProjectName, p.config.Environment)

	bucket := p.getStdIn(fmt.Sprintf("S3 Bucket: (%s)", S3Bucket))
	if bucket != "\n" {
		// TODO: Enforce bucket naming restrictions
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html#bucketnamingrules
		p.config.S3Bucket = bucket
	}

	p.writeConfigFile(p.config)
}

func (p *Project) DeployAWS() {
	p.config = p.mapJSONConfigToStruct()
	lambda := cloud.NewLambda()
	p.cloud = lambda
	p.cloud.CheckPermissions()
	p.Package()
	lambda.Deploy()
}

func (p *Project) Package() {
	fmt.Println("Packaging bulaba project...")
	cwd, err := os.Getwd()
	if err != nil {
		utils.BulabaException(err)
	}

	archiveFile := fmt.Sprintf("%s-package.zip", p.config.ProjectName)
	archivePath := path.Join(cwd, archiveFile)

	venv := p.getVirtualEnvironment()
	sitePackages := path.Join(venv, "lib", p.config.PythonVersion, "site-packages")
	if runtime.GOOS == "windows" {
		sitePackages = path.Join(venv, "Lib", "site-packages")
	}

	tempDir, err := os.MkdirTemp(os.TempDir(), "bulaba")
	if err != nil {
		utils.BulabaException(err)
	}

	p.copyNecessaryFilesToTempDir(cwd, tempDir)
	p.copyNecessaryFilesToTempDir(sitePackages, tempDir)
	p.cloud.CreateFunction(fmt.Sprintf("%shandler.py", tempDir))

	p.archivePackage(archivePath, tempDir)
}

func (p *Project) mapJSONConfigToStruct() *Config {
	fmt.Println("Fetching configuration file...")
	data, err := os.ReadFile(InitFilename)
	if err != nil {
		var pErr *os.PathError
		if errors.As(err, &pErr) {
			msg := fmt.Sprintf("Unable to locate %s file. Please run 'bulaba init' first",
				InitFilename)
			utils.BulabaException(msg)
		}
		utils.BulabaException(err.Error())
	}

	err = json.Unmarshal(data, p.config)
	if err != nil {
		utils.BulabaException(err.Error())
	}
	return p.config
}

func (p *Project) getPythonVersion() string {
	pipVersion, err := p.getShellCommandOutput("pip", "-V")
	if err != nil {
		utils.BulabaException(err)	
	}
	s := strings.Split(pipVersion, " ")
	pythonVersion := strings.ReplaceAll(s[len(s)-1], ")", "")
	pythonVersion = strings.TrimSpace(pythonVersion)
	return fmt.Sprintf("python%s", pythonVersion)
}

func (p *Project) getVirtualEnvironment() string {
	venv := os.Getenv("VIRTUAL_ENV")
	if venv != "" {
		return strings.TrimSpace(venv)
	}

	_, err := p.getShellCommandOutput("pyenv")
	if err != nil {
		pyenvRoot, err := p.getShellCommandOutput("pyenv", "root")
		if err != nil {
			utils.BulabaException(err)
		}
		pyenvRoot = strings.TrimSpace(pyenvRoot)

		pyenvVersionName, err := p.getShellCommandOutput("pyenv", "version-name")
		if err != nil {
			utils.BulabaException(err)
		}
		pyenvVersionName = strings.TrimSpace(pyenvVersionName)
		venv = path.Join(pyenvRoot, "versions", pyenvVersionName)
		return venv
	}
	return ""
}

func (p *Project) getShellCommandOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	out, err := cmd.Output()
	return string(out), err
}

func (p *Project) archivePackage(archivePath, dir string) {
	fmt.Println("Archiving package...")
	archive, err := os.Create(archivePath)
	if err != nil {
		utils.BulabaException(err)
	}
	defer archive.Close()

	writer := zip.NewWriter(archive)
	defer writer.Close()

	walker := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		w, err := writer.Create(path)
		if err != nil {
			return err
		}

		if _, err := io.Copy(w, f); err != nil {
			return err
		}
		return nil
	}
	err = filepath.WalkDir(dir, walker)
	if err != nil {
		utils.BulabaException(err)
	}
}

func (p *Project) copyNecessaryFilesToTempDir(src, dest string) {
	opt := cp.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			for _, file := range excludedFiles {
				return strings.HasSuffix(src, file), nil
			}
			return false, nil
		},
	}
	err := cp.Copy(src, dest, opt)
	if err != nil {
		utils.BulabaException(err)
	}
}

func (p *Project) writeConfigFile(c *Config) {
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
