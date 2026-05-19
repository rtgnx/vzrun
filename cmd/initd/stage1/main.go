//go:build linux

package main

import (
	"os"
	"syscall"

	_ "embed"

	"golang.org/x/sys/unix"
)

//go:generate env GOOS=linux GOARCH=arm64 go generate ../stage2
//go:generate env GOOS=linux GOARCH=arm64 go build -o ./stage2.bin ../stage2
//go:embed stage2.bin
var stage2 []byte

func main() {
	must(0, os.Mkdir("/proc", 0555))
	must(0, os.Mkdir("/sys", 0555))
	must(0, os.Mkdir("/dev", 0755))
	must(0, os.Mkdir("/run", 0755))

	must(0, unix.Mount("tmpfs", "/run", "tmpfs", 0, ""))

	must(0, os.Mkdir("/new_root", 0755))
	must(0, os.Mkdir("/new_root.ro", 0755))
	must(0, os.MkdirAll("/run/overlay/upper", 0755))
	must(0, os.MkdirAll("/run/overlay/work", 0755))

	must(0, unix.Mount("proc", "/proc", "proc", 0, ""))
	must(0, unix.Mount("sysfs", "/sys", "sysfs", 0, ""))
	must(0, unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, ""))
	must(0, unix.Mount("/dev/vda", "/new_root.ro", "erofs", 0, ""))

	must(0, unix.Mount(
		"overlay",
		"/new_root",
		"overlay",
		0,
		"lowerdir=/new_root.ro,upperdir=/run/overlay/upper,workdir=/run/overlay/work",
	))
	must(0, os.Mkdir("/new_root/proc", 0555))
	must(0, os.Mkdir("/new_root/sys", 0555))
	must(0, os.Mkdir("/new_root/dev", 0755))
	must(0, os.Mkdir("/new_root/run", 0755))

	must(0, unix.Mount("/proc", "/new_root/proc", "", unix.MS_MOVE, ""))
	must(0, unix.Mount("/sys", "/new_root/sys", "", unix.MS_MOVE, ""))
	must(0, unix.Mount("/dev", "/new_root/dev", "", unix.MS_MOVE, ""))
	must(0, unix.Mount("/run", "/new_root/run", "", unix.MS_MOVE, ""))
	must(0, unix.Chdir("/new_root"))
	must(0, os.Mkdir("old_root", 0700))
	must(0, unix.PivotRoot(".", "old_root"))
	must(0, unix.Chdir("/"))
	must(0, unix.Unmount("/old_root", unix.MNT_DETACH))
	must(0, os.Remove("/old_root"))

	fd := must(os.OpenFile(
		"/.stage2", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700,
	))

	must(fd.Write(stage2))

	fd.Close()

	os.Setenv("PATH", "/bin:/usr/bin:/sbin:/usr/sbin")

	must(0, syscall.Exec("/.stage2", []string{"/.stage2"}, os.Environ()))
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
