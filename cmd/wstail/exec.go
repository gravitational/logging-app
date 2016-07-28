package main

import (
	"os/exec"

	"github.com/gravitational/trace"
)

func kubectlCmd(name string, args ...string) (out []byte, err error) {
	cmd := exec.Command("/usr/bin/local/kubectl", args...)
	if out, err = cmd.CombinedOutput(); err != nil {
		return out, trace.Wrap(err)
	}
	return out, nil
}
