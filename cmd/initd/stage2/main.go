//go:build linux

package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/rtgnx/vzrun/internal/initd"
)

func main() {
	if err := initd.ConfigureDHCP(context.Background()); err != nil {
		log.Fatal(err.Error())
	}

	tc := new(initd.InitConfig)
	cfg, err := initd.ConfigFromKernelCmdline()
	if err != nil {
		log.Fatal(err)
	}

	if err := tc.Decode(cfg); err != nil {
		log.Fatal(err)
	}

	for k, v := range tc.EnvVars {
		os.Setenv(k, v)
	}

	log.Printf("exec: cmd=%s, args=%+v", tc.ExecCommand, tc.ExecArgs)

	cmd, err := exec.LookPath(tc.ExecCommand)
	if err != nil {
		log.Fatal(err)
	}

	err = syscall.Exec(
		cmd, append([]string{cmd}, tc.ExecArgs...), os.Environ(),
	)

	if err != nil {
		log.Fatal(err)
	}
}
