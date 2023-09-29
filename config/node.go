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
func NewNodeRuntime(cmd utils.ShellCommand) RuntimeInterface {
	runtime := &Runtime{cmd, RuntimeNode, DefaultNodeVersion, ""}
	n := &Node{runtime}
	version, err := n.getVersion()
	if err != nil {
		log.Debug(fmt.Sprintf("encountered an error while getting nodejs version. Default to v%s", DefaultNodeVersion))
		return n
	}
	n.Version = version
	return n
}

// Gets the nodejs version
func (n *Node) getVersion() (string, error) {
	log.Debug("getting nodejs version...")
	nodeVersion, err := n.RunCommand("node", "-v")
	if err != nil {
		return "", err
	}
	nodeVersion = strings.TrimSpace(nodeVersion[1:])
	return nodeVersion, nil
}

// lambdaRuntime is the name of the nodejs runtime as specified by AWS Lambda
func (n *Node) lambdaRuntime() (string, error) {
	v := strings.Split(n.Version, ".")
	return fmt.Sprintf("%s%s.x", n.Name, v[0]), nil
}
