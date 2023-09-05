package config

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/otiai10/copy"
	"github.com/spatocode/jerm/internal/utils"
)

const (
	AwsPythonConfigDocs = "https://boto3.readthedocs.io/en/latest/guide/quickstart.html#configuration"
)

type Python struct{}

func NewPythonConfig() *Python {
	return &Python{}
}

func (p *Python) getVersion() (string, error) {
	pythonVersion, err := utils.GetShellCommandOutput("python", "-V")
	if err != nil || strings.Contains(pythonVersion, " 2.") {
		pythonVersion, err = utils.GetShellCommandOutput("python3", "-V")
	}
	s := strings.Split(pythonVersion, " ")
	version := s[len(s)-1]
	return version, err
}

// getVirtualEnvironment gets the python active virtual environment
func (p *Python) getVirtualEnvironment() (string, error) {
	venv := os.Getenv("VIRTUAL_ENV")
	if venv != "" {
		return strings.TrimSpace(venv), nil
	}

	_, err := utils.GetShellCommandOutput("pyenv")
	if err != nil {
		pyenvRoot, err := utils.GetShellCommandOutput("pyenv", "root")
		if err != nil {
			return "", err
		}
		pyenvRoot = strings.TrimSpace(pyenvRoot)

		pyenvVersionName, err := utils.GetShellCommandOutput("pyenv", "version-name")
		if err != nil {
			return "", err
		}
		pyenvVersionName = strings.TrimSpace(pyenvVersionName)
		venv = path.Join(pyenvRoot, "versions", pyenvVersionName)
		return venv, nil
	}
	return "", nil
}

// Build builds the Python deployment package
func (p *Python) Build(config *Config) (string, error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "jerm-python")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	venv, err := p.getVirtualEnvironment()
	if err != nil {
		err = p.installRequirements()
		return tempDir, err
	}

	sitePackages := path.Join(venv, "lib", DetectRuntime().Version, "site-packages")
	if runtime.GOOS == "windows" {
		sitePackages = path.Join(venv, "Lib", "site-packages")
	}

	p.installNecessaryDependencies(tempDir)
	p.copyNecessaryFilesToTempDir(config.Dir, tempDir)
	p.copyNecessaryFilesToTempDir(sitePackages, tempDir)
	return filepath.Join(tempDir, "handler.py"), nil
}

func (p *Python) copyNecessaryFilesToTempDir(src, dest string) {
	opt := copy.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			for _, glob := range defaultIgnoredGlobs {
				matchFile := strings.HasSuffix(src, glob) ||
					strings.HasPrefix(src, glob) || src == glob
				if matchFile {
					return matchFile, nil
				}
			}
			return false, nil
		},
	}
	err := copy.Copy(src, dest, opt)
	if err != nil {
		// utils.JermException(err)
	}
}

// installRequirements installs requirements listed in requirements.txt file
func (p *Python) installRequirements() error {
	return nil
}

// installNecessaryDependencies installs dependencies needed to run serverless Python
func (p *Python) installNecessaryDependencies(tempDir string) error {
	dependencies := map[string]string{"lambda-wsgi-adapter": "0.1.1"}
	for project, version := range dependencies {
		url := fmt.Sprintf("https://pypi.org/pypi/%s/json", project)
		res, err := utils.Request(url)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		data := make(map[string]interface{})
		err = json.Unmarshal(b, &data)
		if err != nil {
			return err
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
	return nil
}

// downloadDependencies downloads dependencies from pypi
func (p *Python) downloadDependencies(url, filename, tempDir string) error {
	res, err := utils.Request(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	temp, err := os.MkdirTemp(os.TempDir(), "jerm-dep")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	filenamePath := filepath.Join(temp, filename)
	file, err := os.Create(filenamePath)
	if err != nil {
		return err
	}
	defer file.Close()
	file.Write(b)

	if err := p.extractWheel(filenamePath, tempDir); err != nil {
		return err
	}

	return nil
}

func (p *Python) extractWheel(wheelPath, outputDir string) error {
	reader, err := zip.OpenReader(wheelPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		os.MkdirAll(filepath.Join(outputDir, filepath.Dir(file.Name)), 0755)
		if err != nil {
			return err
		}

		extractedFile, err := os.Create(filepath.Join(outputDir, file.Name))
		if err != nil {
			return err
		}
		defer extractedFile.Close()

		zippedFile, err := file.Open()
		if err != nil {
			return err
		}
		defer zippedFile.Close()

		if _, err = io.Copy(extractedFile, zippedFile); err != nil {
			return err
		}
	}

	return nil
}

func (p *Python) getDjangoSettings() string {
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
		// utils.JermException(err)
	}
	return djangoPath
}
