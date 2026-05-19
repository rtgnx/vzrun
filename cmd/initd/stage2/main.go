//go:build linux

package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/rtgnx/vzrun/internal/initd"
	"golang.org/x/sys/unix"
)

//go:generate env GOOS=linux GOARCH=arm64 go build -o ./stage3.bin ../stage3
//go:embed stage3.bin
var stage3 []byte

func main() {
	if err := initd.ConfigureDHCP(context.Background()); err != nil {
		log.Fatal(err.Error())
	}

	fd, err := os.OpenFile("/.stage3", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := fd.Write(stage3); err != nil {
		fd.Close()
		log.Fatalf("failed to write stage3: %v", err)
	}

	fd.Close()

	defer unix.Sync()
	defer unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)

	cmd := exec.Command("/.stage3")
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		log.Print(err)
	}
}
