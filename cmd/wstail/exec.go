package main

import (
	"os/exec"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

func kubectlCmd(name string, args ...string) (out []byte, err error) {
	cmd := exec.Command("/usr/bin/local/kubectl", args...)
	log.Infof("exec %v", cmd)
	if out, err = cmd.CombinedOutput(); err != nil {
		return out, trace.Wrap(err)
	}
	return out, nil
}
