//go:build linux

package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/rtgnx/vzrun/internal/initd"
	"golang.org/x/sys/unix"
)

func main() {
	if err := os.Mkdir(`/proc`, 0555); err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	must(0, unix.Mount("proc", "/proc", "proc", 0, ""))
	//	must(0, unix.Mount("sysfs", "/sys", "sysfs", 0, ""))
	//must(0, unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, ""))
	//must(0, unix.Mount("/dev/vda", "/new_root.ro", "erofs", 0, ""))

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
	cmd, err := exec.LookPath(tc.ExecCommand)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(syscall.Exec(
		cmd, append([]string{cmd}, tc.ExecArgs...), os.Environ(),
	))

}

func must[T any](v T, err error) T {
	if err != nil {
		if os.IsExist(err) {
			return v
		}
		panic(err.Error())
	}

	return v
}
