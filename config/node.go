package config

import (
	"fmt"
	"strings"

	"github.com/spatocode/jerm/internal/log"
	"github.com/spatocode/jerm/internal/utils"
)

// NodeJS runtime
type Node struct {
	*Runtime
}

// NewNodeConfig instantiates a new NodeJS runtime
func NewNodeRuntime() RuntimeInterface {
	runtime := &Runtime{}
	n := &Node{runtime}
	n.Name = RuntimeNode
	version, err := n.getVersion()
	if err != nil {
		log.Debug(fmt.Sprintf("encountered an error while getting nodejs version. Default to %s", DefaultNodeVersion))
		n.Version = DefaultNodeVersion
		return n
	}
	n.Version = version
	return n
}

// Gets the nodejs version
func (n *Node) getVersion() (string, error) {
	log.Debug("getting nodejs version...")
	nodeVersion, err := utils.GetShellCommandOutput("node", "-v")
	if err != nil {
		return "", err
	}
	nodeVersion = nodeVersion[1:]
	return nodeVersion, nil
}

// Builds the nodejs deployment package
func (n *Node) Build(config *Config, functionContent string) (string, string, error) {
	return "", "", nil
}

// lambdaRuntime is the name of the nodejs runtime as specified by AWS Lambda
func (n *Node) lambdaRuntime() (string, error) {
	v := strings.Split(n.Version, ".")
	return fmt.Sprintf("%s%s.x", n.Name, v[0]), nil
}
