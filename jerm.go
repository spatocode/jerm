package jerm

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spatocode/jerm/config"
	"github.com/spatocode/jerm/internal/log"
)

// Config is the Jerm configuration
type Config config.Config

const (
	Version           = "0.0.2"
	DefaultConfigFile = "jerm.json"
	ArchiveFile       = "jerm.zip"
)

var (
	ReadConfig  = config.ReadConfig
	ParseConfig = config.ParseConfig
)

// Project holds details of a Jerm project
type Project struct {
	config *config.Config
	cloud  CloudPlatform
}

// New creates a new Jerm project
func New(cfg *config.Config) (*Project, error) {
	p := &Project{config: cfg}
	return p, nil
}

// Logs shows the deployment logs
func (p *Project) Logs() {
	log.PrintInfo("Fetching logs...")
	p.cloud.Logs()
}

// SetPlatform sets the cloud platform
func (p *Project) SetPlatform(cloud CloudPlatform) {
	p.cloud = cloud
}

// Deploy deploys the project to the cloud
func (p *Project) Deploy() {
	log.PrintfInfo("Deploying project %s...", p.config.Name)

	start := time.Now()

	deployInfo := func(size int64, start time.Time, buildDuration time.Duration) {
		deployDuration := time.Since(start)
		fmt.Printf("%s %s %v %s, (%s)\n", log.Magenta("build:"), log.Green("completed"), log.White(size/2048), log.Blue("MB"), log.White(buildDuration.Round(time.Second)))
		fmt.Printf("%s %s (%s)\n", log.Magenta("deploy:"), log.Green("completed"), log.White(deployDuration.Round(time.Second)))
	}

	file, size, err := p.packageProject()
	if err != nil {
		log.PrintError(err.Error())
		return
	}
	defer os.RemoveAll(*file)

	buildDuration := time.Since(start)

	alreadyDeployed, err := p.cloud.Deploy(*file)
	if err != nil {
		log.PrintError(err.Error())
		return
	}

	if alreadyDeployed {
		log.Debug("project already deployed. updating...")
		err = p.Update(file)
		if err != nil {
			log.PrintError(err.Error())
			return
		}
		deployInfo(size, start, buildDuration)
		return
	}

	deployInfo(size, start, buildDuration)
}

// Update updates the deployed project
func (p *Project) Update(zipPath *string) error {
	log.Debug("updating deployment...")
	var err error
	file := zipPath

	if zipPath == nil {
		file, _, err = p.packageProject()
		if err != nil {
			return err
		}
	}

	err = p.cloud.Update(*file)
	if err != nil {
		return err
	}
	defer os.RemoveAll(*file)

	return nil
}

// Undeploy terminates a deployment
func (p *Project) Undeploy() {
	log.PrintInfo("Undeploying project...")

	start := time.Now()
	err := p.cloud.Undeploy()
	if err != nil {
		log.PrintError(err.Error())
		return
	}

	duration := time.Since(start)
	fmt.Printf("%s %s (%s)\n", log.Magenta("undeploy:"), log.Green("completed"), log.White(duration.Round(time.Second)))
}

// Rollback rolls back a deployment to previous versions
func (p *Project) Rollback(steps int) {
	log.PrintInfo("Rolling back deployment...")

	start := time.Now()
	err := p.cloud.Rollback(steps)
	if err != nil {
		log.PrintError(err.Error())
	}

	duration := time.Since(start)
	fmt.Printf("%s %s (%s)\n", log.Magenta("rollback:"), log.Green("completed"), log.White(duration.Round(time.Second)))
}

// packageProject packages a project for deployment
func (p *Project) packageProject() (*string, int64, error) {
	log.Debug("packaging project...")
	dir, err := p.cloud.Build()
	if err != nil {
		return nil, 0, err
	}

	archivePath := path.Join(p.config.Dir, ArchiveFile)
	size, err := p.archivePackage(archivePath, dir)
	return &archivePath, size, err
}

// archivePackage creates an archive file from a project
func (p *Project) archivePackage(archivePath, dir string) (int64, error) {
	log.Debug("archiving package...")
	archive, err := os.Create(archivePath)
	if err != nil {
		return 0, err
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
		return 0, err
	}

	info, err := archive.Stat()

	return info.Size(), err
}
