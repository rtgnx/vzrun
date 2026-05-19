package oci

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/erofs/go-erofs"
)

type ProgressFunc func(int64)

func NopProgress(int64) {}

func UntarToEroFS(ctx context.Context, img *erofs.Writer, r io.Reader, progress ProgressFunc) error {
	if progress == nil {
		progress = NopProgress
	}

	var extracted int64
	tr := tar.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		hdr, err := tr.Next()
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

		switch hdr.Typeflag {
		case tar.TypeDir:
			err = img.Mkdir(name, hdr.FileInfo().Mode())
		case tar.TypeReg, tar.TypeRegA:
			err = writeErofsFile(ctx, img, name, tr, hdr.Size)
		case tar.TypeSymlink:
			err = img.Symlink(hdr.Linkname, name)
		case tar.TypeLink:
			err = copyErofsHardlink(ctx, img, name, hdr.Linkname)
		default:
			continue
		}
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}

		if err := applyErofsMetadata(img, name, hdr); err != nil {
			return fmt.Errorf("apply metadata for %s: %w", name, err)
		}

		if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA {
			extracted += hdr.Size
			progress(extracted)
		}
	}
}

func writeErofsFile(ctx context.Context, img *erofs.Writer, name string, r io.Reader, size int64) error {
	f, err := img.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(f, contextReader{ctx: ctx, r: r}, size); err != nil {
		return err
	}
	return f.Close()
}

func copyErofsHardlink(ctx context.Context, img *erofs.Writer, name, target string) error {
	target, err := cleanArchiveName(target)
	if err != nil {
		return err
	}

	src, err := img.Open(target)
	if err != nil {
		return fmt.Errorf("open hard link target %s: %w", target, err)
	}
	defer src.Close()

	dst, err := img.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, contextReader{ctx: ctx, r: src}); err != nil {
		return err
	}
	return dst.Close()
}

func applyErofsMetadata(img *erofs.Writer, name string, hdr *tar.Header) error {
	if err := img.Chmod(name, hdr.FileInfo().Mode()); err != nil {
		return err
	}
	if err := img.Chown(name, hdr.Uid, hdr.Gid); err != nil {
		return err
	}
	if err := img.Chtimes(name, hdr.AccessTime, hdr.ModTime); err != nil {
		return err
	}
	for key, value := range hdr.Xattrs {
		if err := img.Setxattr(name, key, value); err != nil {
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
