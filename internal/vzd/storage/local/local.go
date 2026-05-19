package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	boot "github.com/rtgnx/vzrun/internal/initd/boot"
	"github.com/rtgnx/vzrun/internal/vzd/storage"
	"github.com/rtgnx/vzrun/pkg/types"
	"go.yaml.in/yaml/v2"
)

/*
 * Data directory layout
 * - images/*.tar
 * - vm/*.{img,yml}
 * - kernel/{default,initrd}
 */

type Local struct {
	root string
}

var _ storage.Store = (*Local)(nil)

func New(root string) (*Local, error) {
	storage := &Local{
		root: root,
	}
	return storage, storage.init()
}

func (s *Local) init() error {
	dirs := []string{
		storage.KindImage, storage.KindKernel, storage.KindVM,
	}
	for _, dir := range dirs {
		dirPath := filepath.Join(s.root, dir)
		if err := os.MkdirAll(dirPath, 0750); err != nil {
			if os.IsExist(err) {
				continue
			}
			return err
		}
	}
	kernelPath := filepath.Join(s.root, storage.KindKernel, "default")
	initrdPath := filepath.Join(s.root, storage.KindKernel, "initrd")

	return errors.Join(
		os.WriteFile(kernelPath, boot.KernelBin(), 0600),
		os.WriteFile(initrdPath, boot.InitrdBin(), 0600),
	)
}

func (s *Local) KernelPath() string {
	return filepath.Join(s.root, storage.KindKernel, "default")
}

func (s *Local) InitrdPath() string {
	return filepath.Join(s.root, storage.KindKernel, "initrd")
}

func (s *Local) Open(ctx context.Context, kind, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fp, err := s.path(kind, key)
	if err != nil {
		return nil, err
	}
	return os.Open(fp)
}

func (s *Local) Create(ctx context.Context, kind, key string) (*os.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fp, err := s.path(kind, key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
		return nil, err
	}
	return os.OpenFile(fp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
}

func (s *Local) Remove(ctx context.Context, kind, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fp, err := s.path(kind, key)
	if err != nil {
		return err
	}
	err = os.Remove(fp)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Local) Lookup(ctx context.Context, kind, key string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	fp, err := s.path(kind, key)
	if err != nil {
		return "", err
	}
	_, err = os.Stat(fp)
	if err != nil {
		return "", err
	}
	return fp, nil
}

func (s *Local) path(kind, key string) (string, error) {
	kind, err := cleanKey(kind)
	if err != nil {
		return "", err
	}
	key, err = cleanKey(key)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, kind, filepath.FromSlash(key)), nil
}

func cleanKey(key string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(key, "./"))
	if clean == "." || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid storage key: %s", key)
	}
	return clean, nil
}

func (s *Local) PutVMConfig(name string, cfg types.VM) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(s.root, "vm", fmt.Sprintf("%s.yml", name))

	return os.WriteFile(cfgPath, b, 0600)
}

func (s *Local) GetVMConfig(name string) (types.VM, error) {
	cfg := new(types.VM)
	cfgPath := filepath.Join(s.root, "vm", fmt.Sprintf("%s.yml", name))
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return *cfg, err
	}
	err = yaml.Unmarshal(b, cfg)
	return *cfg, err
}

func (s *Local) ListVMConfigs() ([]types.VM, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "vm"))
	if err != nil {
		return nil, err
	}

	var configs []types.VM
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-len(".yml")]
		cfg, err := s.GetVMConfig(name)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

func (s *Local) DeleteVM(name string) error {
	return errors.Join(
		removeIfExists(filepath.Join(s.root, "vm", fmt.Sprintf("%s.yml", name))),
		removeIfExists(filepath.Join(s.root, storage.KindVM, storage.VMRootDiskKey(name))),
	)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
