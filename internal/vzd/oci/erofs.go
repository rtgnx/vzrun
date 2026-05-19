package oci

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/erofs/go-erofs"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rtgnx/vzrun/internal/vzd/storage"
)

type RootDiskBuilder struct {
	store storage.Store
}

func NewRootDiskBuilder(store storage.Store) RootDiskBuilder {
	return RootDiskBuilder{store: store}
}

func (b RootDiskBuilder) NewRootDisk(ctx context.Context, img CachedImage, name string) error {
	key := storage.VMRootDiskKey(name)
	_, err := b.store.Lookup(ctx, storage.KindVM, key)
	if err == nil {
		return nil
	}

	if !os.IsNotExist(err) {
		return err
	}

	ociImage, err := tarball.Image(
		func() (io.ReadCloser, error) {
			return img.Open(ctx)
		},
		nil,
	)
	if err != nil {
		return err
	}

	rootfs := mutate.Extract(ociImage)
	defer rootfs.Close()

	fd, err := b.store.Create(ctx, storage.KindVM, key)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			fd.Close()
			_ = b.store.Remove(ctx, storage.KindVM, key)
		}
	}()

	now := time.Now()
	wr := erofs.Create(fd, erofs.WithBuildTime(uint64(now.Unix()), uint32(now.Nanosecond())))

	if err := erofsFromTar(ctx, wr, tar.NewReader(rootfs)); err != nil {
		return err
	}
	if err := wr.Close(); err != nil {
		return err
	}
	if err := fd.Close(); err != nil {
		return err
	}
	committed = true
	return nil
}

func erofsFromTar(ctx context.Context, w *erofs.Writer, r *tar.Reader) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		hdr, err := r.Next()
		if err == io.EOF {
			return ctx.Err()
		}
		if err != nil {
			return err
		}

		name, err := cleanArchiveName(hdr.Name)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}

		written, err := writeErofsEntry(ctx, w, r, hdr, name)
		if err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		if !written {
			continue
		}

		if err := applyErofsMetadata(w, name, hdr); err != nil {
			return fmt.Errorf("apply metadata for %s: %w", name, err)
		}
	}
}

func writeErofsEntry(ctx context.Context, w *erofs.Writer, r io.Reader, hdr *tar.Header, name string) (bool, error) {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return true, w.Mkdir(name, hdr.FileInfo().Mode())
	case tar.TypeReg, tar.TypeRegA:
		return true, writeErofsFile(ctx, w, name, r, hdr.Size)
	case tar.TypeSymlink:
		return true, w.Symlink(hdr.Linkname, name)
	case tar.TypeLink:
		return true, copyErofsHardlink(ctx, w, name, hdr.Linkname)
	default:
		return false, nil
	}
}

func writeErofsFile(ctx context.Context, w *erofs.Writer, name string, r io.Reader, size int64) error {
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(f, contextReader{ctx: ctx, r: r}, size); err != nil {
		return err
	}
	return f.Close()
}

func copyErofsHardlink(ctx context.Context, w *erofs.Writer, name, target string) error {
	target, err := cleanArchiveName(target)
	if err != nil {
		return err
	}

	src, err := w.Open(target)
	if err != nil {
		return fmt.Errorf("open hard link target %s: %w", target, err)
	}
	defer src.Close()

	dst, err := w.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, contextReader{ctx: ctx, r: src}); err != nil {
		return err
	}
	return dst.Close()
}

func applyErofsMetadata(w *erofs.Writer, name string, hdr *tar.Header) error {
	if err := w.Chmod(name, hdr.FileInfo().Mode()); err != nil {
		return err
	}
	if err := w.Chown(name, hdr.Uid, hdr.Gid); err != nil {
		return err
	}
	if err := w.Chtimes(name, hdr.AccessTime, hdr.ModTime); err != nil {
		return err
	}
	for key, value := range hdr.Xattrs {
		if err := w.Setxattr(name, key, value); err != nil {
			return err
		}
	}
	return nil
}

func cleanArchiveName(name string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(name, "./"))
	if clean == "." {
		return "", nil
	}
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path escapes archive root: %s", name)
	}
	return clean, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
