package storage

import (
	"context"
	"io"
	"os"
)

const (
	KindImage  = "image"
	KindVM     = "vm"
	KindKernel = "kernel"
	// Example Keys
	// KindImage, key = sha256:XXXXX...
	// KindVM, key = %s/root.img  - root disk
	// KindVM, key = %s/config.yml - vm config
)

func VMRootDiskKey(name string) string {
	return name + "/root.img"
}

type Store interface {
	Open(ctx context.Context, kind, key string) (io.ReadCloser, error)
	Create(ctx context.Context, kind, key string) (*os.File, error)
	Remove(ctx context.Context, kind, key string) error
	Lookup(ctx context.Context, kind, key string) (string, error)
}
