package jerm

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/utils"
)

type Config config.Config

const (
	DefaultConfigFile       = "jerm.json"
	ArchiveFile             = "jerm.zip"
)

var (
	ReadConfig  = config.ReadConfig
	ParseConfig = config.ParseConfig
)

type Project struct {
	config *config.Config
	cloud  Platform
}

func New(cfg *config.Config) (*Project, error) {
	p := &Project{config: cfg}
	return p, nil
}

func (p *Project) Logs() {

}

// SetPlatform sets the cloud platform
func (p *Project) SetPlatform(cloud Platform) {
	p.cloud = cloud
}

// Deploy deploys the project to the cloud
func (p *Project) Deploy() {
	utils.LogInfo("Deploying project %s...", p.config.Name)
	file, err := p.packageProject()
	if err != nil {
		slog.Error(err.Error())
		return
	}

	alreadyDeployed, err := p.cloud.Deploy(*file)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	if alreadyDeployed {
		utils.LogInfo("Project already deployed. Updating...")
		p.Update(file)
		return
	}

	os.RemoveAll(filepath.Dir(*file))

	fmt.Println("Done!")
}

func (p *Project) Update(zipPath *string) {
	var err error
	file := zipPath

	if zipPath == nil {
		file, err = p.packageProject()
		if err != nil {
			slog.Error(err.Error())
			return
		}
	}

	p.cloud.Update(*file)
	os.RemoveAll(filepath.Dir(*file))

	fmt.Println("Done!")
}

func (p *Project) Undeploy() {
	p.cloud.Undeploy()
	fmt.Println("Done!")
}

func (p *Project) Rollback() {
	fmt.Println("Rolling back deployment...")
	p.cloud.Rollback()
	p.cloud.Logs()
}

func (p *Project) packageProject() (*string, error) {
	dir, err := p.cloud.Build()
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	tempBuildDir, err := os.MkdirTemp(os.TempDir(), "jerm-build")
	if err != nil {
		return nil, err
	}

	archivePath := path.Join(tempBuildDir, ArchiveFile)
	p.archivePackage(archivePath, dir)
	return &archivePath, nil
}

func (p *Project) archivePackage(archivePath, dir string) {
	slog.Debug("Archiving package...")
	archive, err := os.Create(archivePath)
	if err != nil {
		// utils.JermException(err)
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
		// utils.JermException(err)
	}
}

func (c *Project) GetStdIn(prompt string) (string, error) {
	if prompt != "" {
		fmt.Println(prompt)
	}
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	fmt.Println()
	return strings.TrimSpace(value), nil
}
