package bin

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	_ "embed"
)

//go:generate env GOOS=linux GOARCH=arm64 go generate ../../../cmd/initd/stage1
//go:generate env GOOS=linux GOARCH=arm64 go build -o init ../../../cmd/initd/stage1
//go:generate sh -c "printf 'init\\n' | cpio -o -H newc > initrd.cpio"

//go:embed initrd.cpio
var initrd []byte

func InitrdBin() []byte {
	return initrd
}

// WriteInitrd writes the smallest vzrun initramfs: /init and the cpio trailer.
func WriteInitrd(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := &cpioWriter{w: f, ino: 1}
	if err := w.writeFile("init", initrd, 0755); err != nil {
		return err
	}
	return w.close()
}

type cpioWriter struct {
	w   io.Writer
	ino uint32
}

func (w *cpioWriter) writeFile(name string, data []byte, mode os.FileMode) error {
	return w.writeEntry(name, uint32(mode)|0100000, int64(len(data)), time.Now(), 1, strings.NewReader(string(data)))
}

func (w *cpioWriter) close() error {
	return w.writeEntry("TRAILER!!!", 0, 0, time.Time{}, 1, nil)
}

func (w *cpioWriter) writeEntry(name string, mode uint32, size int64, modTime time.Time, nlink uint32, body io.Reader) error {
	if size < 0 {
		return fmt.Errorf("negative file size for %s", name)
	}

	var mtime int64
	if !modTime.IsZero() {
		mtime = modTime.Unix()
	}
	header := fmt.Sprintf(
		"070701%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x",
		w.ino,
		mode,
		0,
		0,
		nlink,
		mtime,
		size,
		0,
		0,
		0,
		0,
		len(name)+1,
		0,
	)
	w.ino++

	if _, err := io.WriteString(w.w, header); err != nil {
		return err
	}
	if _, err := io.WriteString(w.w, name); err != nil {
		return err
	}
	if _, err := w.w.Write([]byte{0}); err != nil {
		return err
	}
	if err := writePadding(w.w, len(header)+len(name)+1); err != nil {
		return err
	}
	if size > 0 {
		if body == nil {
			return fmt.Errorf("missing body for %s", name)
		}
		if _, err := io.CopyN(w.w, body, size); err != nil {
			return err
		}
	}
	return writePadding(w.w, int(size))
}

func writePadding(w io.Writer, n int) error {
	pad := (4 - (n % 4)) % 4
	if pad == 0 {
		return nil
	}
	_, err := w.Write([]byte(strings.Repeat("\x00", pad)))
	return err
}
