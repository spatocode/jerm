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

	"golang.org/x/sync/errgroup"

	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

const (
	DefaultPythonFunctionFile = "handler"
)

type Python struct {
	*Runtime
}

// NewPythonConfig instantiates a new Python runtime
func NewPythonRuntime(cmd utils.ShellCommand) RuntimeInterface {
	runtime := &Runtime{cmd, RuntimePython, DefaultPythonVersion}
	p := &Python{runtime}
	version, err := p.getVersion()
	if err != nil {
		log.Debug(fmt.Sprintf("encountered an error while getting python version. Default to %s", DefaultPythonVersion))
		return p
	}
	p.Version = version
	return p
}

// Entry is the directory where the cloud function handler resides.
// The directory can be a file.
func (p *Python) Entry() string {
	switch {
	case p.IsDjango():
		workDir, _ := os.Getwd()
		entry, err := p.getDjangoProject(workDir)
		if err != nil {
			log.Debug(err.Error())
		}
		return entry
	default:
		return ""
	}
}

// Gets the python version
func (p *Python) getVersion() (string, error) {
	log.Debug("getting python version...")
	pythonVersion, err := p.RunCommand("python", "-V")
	if err != nil || strings.Contains(pythonVersion, " 2.") {
		pythonVersion, err = p.RunCommand("python3", "-V")
		if err != nil {
			return "", err
		}
	}
	s := strings.Split(pythonVersion, " ")
	version := strings.TrimSpace(s[len(s)-1])
	return version, err
}

// Gets the python active virtual environment
func (p *Python) getVirtualEnvironment() (string, error) {
	venv := os.Getenv("VIRTUAL_ENV")
	if venv != "" {
		return strings.TrimSpace(venv), nil
	}

	pyenvRoot, err := p.RunCommand("pyenv", "root")
	if err != nil {
		return "", err
	}
	pyenvRoot = strings.TrimSpace(pyenvRoot)

	pyenvVersionName, err := p.RunCommand("pyenv", "version-name")
	if err != nil {
		return "", err
	}
	pyenvVersionName = strings.TrimSpace(pyenvVersionName)
	venv = path.Join(pyenvRoot, "versions", pyenvVersionName)

	return venv, nil
}

// Build builds the Python deployment package
// It returns the package path, the function name and error if any
func (p *Python) Build(config *Config, functionContent string) (string, string, error) {
	function := config.Platform.Handler
	tempDir, err := os.MkdirTemp(os.TempDir(), "jerm-package")
	if err != nil {
		return "", "", err
	}

	handlerFilepath := filepath.Join(tempDir, "handler.py")

	venv, err := p.getVirtualEnvironment()
	if err != nil {
		//TODO: installs requirements listed in requirements.txt file
		return "", "", fmt.Errorf("cannot find a virtual env. Please ensure you're running in a virtual env")
	}

	version := strings.Split(p.Version, ".")
	sitePackages := path.Join(venv, "lib", fmt.Sprintf("%s%s.%s", p.Name, version[0], version[1]), "site-packages")
	if runtime.GOOS == "windows" {
		sitePackages = path.Join(venv, "Lib", "site-packages")
	}

	dependencies := map[string]string{
		"lambda-wsgi-adapter": "0.1.1",
	}
	if !utils.FileExists(filepath.Join(sitePackages, "werkzeug")) {
		dependencies["werkzeug"] = "0.16.1"
	}

	err = p.installNecessaryDependencies(tempDir, sitePackages, dependencies)
	if err != nil {
		return "", "", err
	}

	err = p.copyNecessaryFilesToPackageDir(config.Dir, tempDir, jermIgnoreFile)
	if err != nil {
		return "", "", err
	}

	err = p.copyNecessaryFilesToPackageDir(sitePackages, tempDir, jermIgnoreFile)
	if err != nil {
		return "", "", err
	}

	log.Debug(fmt.Sprintf("built Python deployment package at %s", tempDir))

	if function == "" && p.IsDjango() { // for now it works for Django projects only
		function, err = p.createFunctionEntry(config, functionContent, handlerFilepath)
		if err != nil {
			return "", "", err
		}
	}

	if err != nil {
		return "", "", err
	}
	return tempDir, function, err
}

// createFunctionEntry creates a serverless function handler file
func (p *Python) createFunctionEntry(config *Config, functionContent, file string) (string, error) {
	log.Debug("creating lambda handler...")
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	handler := strings.ReplaceAll(functionContent, ".wsgi", config.Entry+".wsgi")
	_, err = f.Write([]byte(handler))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", DefaultPythonFunctionFile, DefaultServerlessFunction), nil
}

// installRequirements installs requirements listed in requirements.txt file
// func (p *Python) installRequirements(dir string) error {
// 	return nil
// }

// Installs dependencies needed to run serverless Python
func (p *Python) installNecessaryDependencies(dir, sitePackages string, dependencies map[string]string) error {
	log.Debug("installing necessary Python dependencies...")
	var eg errgroup.Group

	for project, version := range dependencies {
		func(dep, ver string) {
			eg.Go(func() error {
				url := fmt.Sprintf("https://pypi.org/pypi/%s/json", dep)
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
				for _, v := range releases[ver].([]interface{}) {
					url := v.(map[string]interface{})["url"].(string)
					filename := v.(map[string]interface{})["filename"].(string)
					if filepath.Ext(filename) == ".whl" {
						err := p.downloadDependencies(url, filename, dir)
						if err != nil {
							return err
						}
					}
				}
				return nil
			})
		}(project, version)
	}

	err := eg.Wait()

	return err
}

// Downloads dependencies from pypi
func (p *Python) downloadDependencies(url, filename, dir string) error {
	log.Debug("downloading dependencies...")
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

	if err := p.extractWheel(filenamePath, dir); err != nil {
		return err
	}

	return nil
}

// Extracts python wheel from wheelPath to outputDir
func (p *Python) extractWheel(wheelPath, outputDir string) error {
	log.Debug("extracting python wheel...")
	var eg errgroup.Group

	reader, err := zip.OpenReader(wheelPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		func(file *zip.File) {
			eg.Go(func() error {
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
				return nil
			})
		}(file)
	}

	err = eg.Wait()

	return err
}

// Check if it's a Django project
func (p *Python) IsDjango() bool {
	return utils.FileExists("manage.py")
}

// Gets the Django project path
func (p *Python) getDjangoProject(dir string) (string, error) {
	// Not the best way for finding django project path
	// Using this naive hack for now
	djangoPath := ""
	walker := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && d.Name() == "settings.py" {
			d := filepath.Dir(path)
			splitPath := strings.Split(d, string(filepath.Separator))
			djangoPath = splitPath[len(splitPath)-1]
			return io.EOF
		}

		return nil
	}
	err := filepath.WalkDir(dir, walker)
	if err != nil {
		if err == io.EOF {
			err = nil
		}
		return djangoPath, err
	}

	return djangoPath, nil
}

// lambdaRuntime is the name of the python runtime as specified by AWS Lambda
func (p *Python) lambdaRuntime() (string, error) {
	v := strings.Split(p.Version, ".")
	return fmt.Sprintf("%s%s.%s", p.Name, v[0], v[1]), nil
}
