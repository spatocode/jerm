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

	"github.com/otiai10/copy"
	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

type Python struct{}

// NewPythonConfig creates a new Python config
func NewPythonConfig() *Python {
	return &Python{}
}

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

// getVersion gets the python version
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

	handlerPath := filepath.Join(tempDir, "handler.py")

	venv, err := p.getVirtualEnvironment()
	if err != nil {
		//TODO: installs requirements listed in requirements.txt file
		return "", fmt.Errorf("cannot find a virtual env. Please ensure you're running in a virtual env")
	}

	version := strings.Split(DetectRuntime().Version, ".")
	sitePackages := path.Join(venv, "lib", fmt.Sprintf("%s%s.%s", DetectRuntime().Name, version[0], version[1]), "site-packages")
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
		return "", err
	}

	err = p.copyNecessaryFilesToTempDir(config.Dir, tempDir)
	if err != nil {
		return "", err
	}

	err = p.copyNecessaryFilesToTempDir(sitePackages, tempDir)
	if err != nil {
		return "", err
	}

	log.Debug(fmt.Sprintf("built Python deployment package at %s", tempDir))

	return handlerPath, err
}

func (p *Python) copyNecessaryFilesToTempDir(src, dest string) error {
	log.Debug("copying necessary Python files...")
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
		return err
	}

	return nil
}

// installRequirements installs requirements listed in requirements.txt file
// func (p *Python) installRequirements(dir string) error {
// 	return nil
// }

// installNecessaryDependencies installs dependencies needed to run serverless Python
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

// downloadDependencies downloads dependencies from pypi
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

func (p *Python) IsDjango() bool {
	return utils.FileExists("manage.py")
}

// gets the Django project path
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
