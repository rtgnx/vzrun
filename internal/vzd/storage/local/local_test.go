package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rtgnx/vzrun/internal/vzd/storage"
)

func TestLocalStoreContract(t *testing.T) {
	ctx := context.Background()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	const (
		kind = storage.KindVM
		key  = "test/root.img"
		want = "rootfs bytes"
	)

	w, err := store.Create(ctx, kind, key)
	if err != nil {
		t.Fatalf("Create(%q, %q) error = %v", kind, key, err)
	}
	if _, err := io.WriteString(w, want); err != nil {
		t.Fatalf("write %q error = %v", key, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer for %q error = %v", key, err)
	}

	fp, err := store.Lookup(ctx, kind, key)
	if err != nil {
		t.Fatalf("Lookup(%q, %q) error = %v", kind, key, err)
	}
	if !strings.HasSuffix(fp, filepath.FromSlash("vm/test/root.img")) {
		t.Fatalf("Lookup(%q, %q) path = %q, want suffix %q", kind, key, fp, filepath.FromSlash("vm/test/root.img"))
	}

	r, err := store.Open(ctx, kind, key)
	if err != nil {
		t.Fatalf("Open(%q, %q) error = %v", kind, key, err)
	}
	got, err := io.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		t.Fatalf("read %q error = %v", key, err)
	}
	if closeErr != nil {
		t.Fatalf("close reader for %q error = %v", key, closeErr)
	}
	if string(got) != want {
		t.Fatalf("Open(%q, %q) content = %q, want %q", kind, key, got, want)
	}

	if err := store.Remove(ctx, kind, key); err != nil {
		t.Fatalf("Remove(%q, %q) error = %v", kind, key, err)
	}
	if _, err := store.Lookup(ctx, kind, key); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lookup(%q, %q) after Remove error = %v, want os.ErrNotExist", kind, key, err)
	}
	if err := store.Remove(ctx, kind, key); err != nil {
		t.Fatalf("Remove(%q, %q) on missing key error = %v, want nil", kind, key, err)
	}
}

func TestLocalStoreRejectsUnsafeKeys(t *testing.T) {
	ctx := context.Background()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name string
		kind string
		key  string
	}{
		{name: "empty kind", kind: "", key: "x"},
		{name: "empty key", kind: storage.KindVM, key: ""},
		{name: "current dir key", kind: storage.KindVM, key: "."},
		{name: "parent key", kind: storage.KindVM, key: "../x"},
		{name: "nested parent key", kind: storage.KindVM, key: "vm/../../x"},
		{name: "absolute key", kind: storage.KindVM, key: "/tmp/x"},
		{name: "parent kind", kind: "../vm", key: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := store.Lookup(ctx, tt.kind, tt.key); err == nil {
				t.Fatalf("Lookup(%q, %q) error = nil, want invalid key error", tt.kind, tt.key)
			}
			if _, err := store.Create(ctx, tt.kind, tt.key); err == nil {
				t.Fatalf("Create(%q, %q) error = nil, want invalid key error", tt.kind, tt.key)
			}
			if _, err := store.Open(ctx, tt.kind, tt.key); err == nil {
				t.Fatalf("Open(%q, %q) error = nil, want invalid key error", tt.kind, tt.key)
			}
			if err := store.Remove(ctx, tt.kind, tt.key); err == nil {
				t.Fatalf("Remove(%q, %q) error = nil, want invalid key error", tt.kind, tt.key)
			}
		})
	}
}

func TestLocalStoreInitializesBootArtifacts(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for name, fp := range map[string]string{
		"kernel": store.KernelPath(),
		"initrd": store.InitrdPath(),
	} {
		t.Run(name, func(t *testing.T) {
			info, err := os.Stat(fp)
			if err != nil {
				t.Fatalf("%s artifact %q stat error = %v", name, fp, err)
			}
			if info.Size() == 0 {
				t.Fatalf("%s artifact %q size = 0, want non-empty", name, fp)
			}
		})
	}
}
