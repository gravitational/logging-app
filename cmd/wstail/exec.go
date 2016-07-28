package main

import (
	"os/exec"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

func kubectlCmd(cmd string, args ...string) (out []byte, err error) {
	call := exec.Command("/usr/local/bin/kubectl", append([]string{cmd}, args...)...)
	log.Infof("exec %v", call)
	if out, err = call.CombinedOutput(); err != nil {
		return out, trace.Wrap(err)
	}
	return out, nil
}
