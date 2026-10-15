//go:build !linux && !darwin

package worker

import "os/exec"

func configureProcess(*exec.Cmd) {}
