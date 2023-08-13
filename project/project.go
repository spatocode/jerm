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
	"time"

	cp "github.com/otiai10/copy"

	"github.com/spatocode/bulaba/cloud"
	"github.com/spatocode/bulaba/utils"
)

var (
	excludedGlobs = []string{
		".zip",
		".exe",
		".git",
		".DS_Store",
		"pip",
		"venv",
		"__pycache__",
		".hg",
		".Python",
		"setuputils",
		"tar.gz",
		".git",
		".vscode",
		"docutils",
		"main",
	}
)

type Project struct {
	config *Config
	cloud  cloud.Platform
}

func LoadProject() *Project {
	workDir, err := os.Getwd()
	if err != nil {
		utils.BulabaException(err.Error())
	}
	splitPath := strings.Split(workDir, "/")
	projectName := splitPath[len(splitPath)-1]
	config := &Config{
		Stage:       Stage,
		Bucket:      fmt.Sprintf("bulaba-%d", time.Now().Unix()),
		ProjectName: projectName,
	}
	p := &Project{
		config: config,
	}
	p.config.PythonVersion = p.getPythonVersion()
	p.config.DjangoSettings = p.getDjangoSettings()
	return p
}

func (p *Project) Init() {
	utils.EnsureProjectExists()
	p.printInitMessage()

	fmt.Println("Specify the name for this deployment stage. [dev, staging, prod]")
	stage := p.getStdIn(fmt.Sprintf("Deployment stage: (%s)", Stage))
	if stage != "\n" {
		fmt.Println(stage != "\n", stage != "")
		// TODO: Check is correct name
		p.config.Stage = strings.TrimSpace(stage)
	}

	p.config.ProjectName = fmt.Sprintf("bulaba-%s-%s", p.config.ProjectName, p.config.Stage)
	awsConfig := cloud.LoadLambda(p.config).AwsConfig
	p.config.Region = awsConfig.Region

	fmt.Println("S3 Bucket is needed to upload your deployment. If you have none yet, we would create one.")
	bucket := p.getStdIn(fmt.Sprintf("S3 Bucket: (%s)", p.config.Bucket))
	if bucket != "\n" {
		// TODO: Enforce bucket naming restrictions
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html#bucketnamingrules
		p.config.Bucket = strings.TrimSpace(bucket)
	}

	p.config.ToJson()
}

func (p *Project) Deploy(cloud cloud.Platform) {
	p.config = p.mapJSONConfigToStruct()
	p.cloud = cloud
	p.cloud.CheckPermissions()
	file := p.packageProject()
	p.cloud.Deploy(file)
	fmt.Println("Done!")
}

func (p *Project) Update(cloud cloud.Platform) {
	p.config = p.mapJSONConfigToStruct()
	p.cloud = cloud
	p.cloud.CheckPermissions()
	file := p.packageProject()
	p.cloud.Update(file)
	fmt.Println("Done!")
}

func (p *Project) Undeploy(cloud cloud.Platform) {
	fmt.Println("Are you sure you want to undeploy? [y/n]")
	ans := p.getStdIn("")
	if strings.TrimSpace(ans) != "y" {
		return
	}
	p.config = p.mapJSONConfigToStruct()
	p.cloud = cloud
	p.cloud.CheckPermissions()
	p.cloud.Undeploy()
	fmt.Println("Done!")
}

func (p *Project) JSONToStruct() *Config {
	return p.mapJSONConfigToStruct()
}

func (p *Project) Rollback() {
	p.config = p.mapJSONConfigToStruct()
	p.cloud = cloud.LoadLambda(p.config)
	p.cloud.Logs()
}

func (p *Project) Package(cloud cloud.Platform) {
	p.cloud = cloud
	p.packageProject()
	fmt.Println("Done!")
}

func (p *Project) packageProject() string {
	fmt.Println("Preparing bulaba project for packaging...")
	cwd, err := os.Getwd()
	if err != nil {
		utils.BulabaException(err)
	}

	archiveFile := fmt.Sprintf("%s%s", p.config.ProjectName, archiveFileSuffix)
	if _, err := os.Stat(archiveFile); err == nil {
		utils.BulabaException("Packaged project detected!")
	}

	fmt.Println("Packaging bulaba project...")

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
	defer os.RemoveAll(tempDir)

	p.installNecessaryDependencies(tempDir)
	p.copyNecessaryFilesToTempDir(cwd, tempDir)
	p.copyNecessaryFilesToTempDir(sitePackages, tempDir)
	f := filepath.Join(tempDir, "handler.py")
	p.cloud.CreateFunctionEntry(f)

	p.archivePackage(archivePath, tempDir)
	return archivePath
}

func (p *Project) installNecessaryDependencies(tempDir string) {
	dependencies := map[string]string{"aws-wsgi": "0.2.7"}
	for project, version := range dependencies {
		url := fmt.Sprintf("https://pypi.org/pypi/%s/json", project)
		res, err := utils.Request(url)
		if err != nil {
			utils.BulabaException(err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			utils.BulabaException(err)
		}
		data := make(map[string]interface{})
		err = json.Unmarshal(b, &data)
		if err != nil {
			utils.BulabaException(err)
		}

		r := data["releases"]
		releases, _ := r.(map[string]interface{})
		for _, v := range releases[version].([]interface{}) {
			url := v.(map[string]interface{})["url"].(string)
			filename := v.(map[string]interface{})["filename"].(string)
			if filepath.Ext(filename) == ".whl" {
				p.downloadDependencies(url, filename, tempDir)
			}
		}
	}
}

func (p *Project) downloadDependencies(url, filename, tempDir string) {
	res, err := utils.Request(url)
	if err != nil {
		utils.BulabaException(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		utils.BulabaException(err)
	}

	temp, err := os.MkdirTemp(os.TempDir(), "bulaba-dep")
	if err != nil {
		utils.BulabaException(err)
	}
	defer os.RemoveAll(temp)

	filenamePath := filepath.Join(temp, filename)
	file, err := os.Create(filenamePath)
	if err != nil {
		utils.BulabaException(err)
	}
	defer file.Close()
	file.Write(b)

	if err := p.extractWheel(filenamePath, tempDir); err != nil {
		utils.BulabaException("Error extracting wheel contents:", err)
	}
}

func (p *Project) extractWheel(wheelPath, outputDir string) error {
	reader, err := zip.OpenReader(wheelPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		os.MkdirAll(filepath.Join(outputDir, filepath.Dir(file.Name)), 0755)
		if err != nil {
			utils.BulabaException(err)
		}

		extractedFile, err := os.Create(filepath.Join(outputDir, file.Name))
		if err != nil {
			utils.BulabaException(err)
		}
		defer extractedFile.Close()

		zippedFile, err := file.Open()
		if err != nil {
			utils.BulabaException(err)
		}
		defer zippedFile.Close()

		if _, err = io.Copy(extractedFile, zippedFile); err != nil {
			utils.BulabaException(err)
		}
	}

	return nil
}

func (p *Project) getDjangoSettings() string {
	workDir, _ := os.Getwd()
	djangoPath := ""
	walker := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, "settings.py") {
			b := filepath.Base(path)
			d := filepath.Dir(path)
			splitPath := strings.Split(d, string(filepath.Separator))
			djangoPath = fmt.Sprintf("%s.%s", splitPath[len(splitPath)-1], b)
		}

		return nil
	}
	err := filepath.WalkDir(workDir, walker)
	if err != nil {
		utils.BulabaException(err)
	}
	return djangoPath
}

func (p *Project) mapJSONConfigToStruct() *Config {
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

		sPath := strings.Split(path, dir)
		zipContentPath := sPath[len(sPath)-1]
		w, err := writer.Create(zipContentPath)
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
			for _, glob := range excludedGlobs {
				matchFile := strings.HasSuffix(src, glob) ||
					strings.HasPrefix(src, glob) || src == glob
				if matchFile {
					return matchFile, nil
				}
			}
			return false, nil
		},
	}
	err := cp.Copy(src, dest, opt)
	if err != nil {
		utils.BulabaException(err)
	}
}

func (c *Project) printInitMessage() {
	fmt.Printf("This utility will walk you through configuring your bulaba deployment by creating a %s file.\n", "bulaba.json")
	fmt.Println()
}

func (c *Project) getStdIn(prompt string) string {
	if prompt != "" {
		fmt.Println(prompt)
	}
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		utils.BulabaException(err.Error())
	}
	fmt.Println()
	return value
}
