package oci

import (
	"archive/tar"
	"bytes"
	"context"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/erofs/go-erofs"
)

func TestCleanArchiveName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "plain path", in: "bin/sh", want: "bin/sh"},
		{name: "dot slash prefix", in: "./etc/hostname", want: "etc/hostname"},
		{name: "root dot", in: ".", want: ""},
		{name: "parent escape", in: "../etc/passwd", wantErr: true},
		{name: "nested parent escape", in: "etc/../../passwd", wantErr: true},
		{name: "absolute path", in: "/etc/passwd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanArchiveName(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("cleanArchiveName(%q) error = nil, want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("cleanArchiveName(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("cleanArchiveName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestErofsFromTarSmoke(t *testing.T) {
	var tarball bytes.Buffer
	tw := tar.NewWriter(&tarball)
	mtime := time.Unix(1700000000, 0)

	writeTarHeader(t, tw, &tar.Header{
		Name:     "etc",
		Typeflag: tar.TypeDir,
		Mode:     0755,
		ModTime:  mtime,
	})
	writeTarFile(t, tw, "etc/hostname", "vzrun\n", 0644, mtime)
	writeTarHeader(t, tw, &tar.Header{
		Name:     "hostname-link",
		Typeflag: tar.TypeSymlink,
		Linkname: "etc/hostname",
		Mode:     0777,
		ModTime:  mtime,
	})
	writeTarHeader(t, tw, &tar.Header{
		Name:     "hostname-copy",
		Typeflag: tar.TypeLink,
		Linkname: "etc/hostname",
		Mode:     0644,
		ModTime:  mtime,
	})
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer error = %v", err)
	}

	out, err := os.CreateTemp(t.TempDir(), "root-*.erofs")
	if err != nil {
		t.Fatalf("create temp erofs image error = %v", err)
	}
	defer out.Close()

	w := erofs.Create(out, erofs.WithBuildTime(uint64(mtime.Unix()), uint32(mtime.Nanosecond())))
	if err := erofsFromTar(context.Background(), w, tar.NewReader(bytes.NewReader(tarball.Bytes()))); err != nil {
		t.Fatalf("erofsFromTar() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close erofs writer error = %v", err)
	}
	if _, err := out.Seek(0, 0); err != nil {
		t.Fatalf("seek erofs image error = %v", err)
	}

	image, err := erofs.Open(out)
	if err != nil {
		t.Fatalf("open generated erofs image error = %v", err)
	}

	data, err := fs.ReadFile(image, "etc/hostname")
	if err != nil {
		t.Fatalf("read etc/hostname from generated erofs error = %v", err)
	}
	if string(data) != "vzrun\n" {
		t.Fatalf("etc/hostname content = %q, want %q", data, "vzrun\n")
	}

	data, err = fs.ReadFile(image, "hostname-copy")
	if err != nil {
		t.Fatalf("read hardlink copy from generated erofs error = %v", err)
	}
	if string(data) != "vzrun\n" {
		t.Fatalf("hostname-copy content = %q, want %q", data, "vzrun\n")
	}

	link, err := fs.ReadLink(image, "hostname-link")
	if err != nil {
		t.Fatalf("read symlink from generated erofs error = %v", err)
	}
	if link != "etc/hostname" {
		t.Fatalf("hostname-link target = %q, want %q", link, "etc/hostname")
	}
}

func TestErofsFromTarRejectsEscapingPath(t *testing.T) {
	var tarball bytes.Buffer
	tw := tar.NewWriter(&tarball)
	writeTarFile(t, tw, "../escape", "bad", 0644, time.Unix(0, 0))
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer error = %v", err)
	}

	out, err := os.CreateTemp(t.TempDir(), "root-*.erofs")
	if err != nil {
		t.Fatalf("create temp erofs image error = %v", err)
	}
	defer out.Close()

	w := erofs.Create(out)
	err = erofsFromTar(context.Background(), w, tar.NewReader(bytes.NewReader(tarball.Bytes())))
	if err == nil {
		t.Fatal("erofsFromTar() error = nil, want escaping path error")
	}
	if !strings.Contains(err.Error(), "path escapes archive root") {
		t.Fatalf("erofsFromTar() error = %v, want path escape error", err)
	}
}

func writeTarFile(t *testing.T, tw *tar.Writer, name, body string, mode int64, modTime time.Time) {
	t.Helper()
	writeTarHeader(t, tw, &tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     mode,
		Size:     int64(len(body)),
		ModTime:  modTime,
	})
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatalf("write tar file %q body error = %v", name, err)
	}
}

func writeTarHeader(t *testing.T, tw *tar.Writer, hdr *tar.Header) {
	t.Helper()
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header %q error = %v", hdr.Name, err)
	}
}
