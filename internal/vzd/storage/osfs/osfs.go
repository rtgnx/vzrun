package osfs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/erofs/go-erofs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	boot "github.com/rtgnx/vzrun/internal/initd/boot"
	"github.com/rtgnx/vzrun/pkg/types"
	"go.yaml.in/yaml/v2"
)

/*
 * Data directory layout
 * - images/*.tar
 * - vm/*.{img,yml}
 * - kernels/Image-vXXXX
 */

type Storage struct {
	root string
}

func New(root string) (*Storage, error) {
	storage := &Storage{
		root: root,
	}
	return storage, storage.init()
}

func (s *Storage) init() error {
	dirs := []string{
		"vm", "images", "kernels",
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
	kernelPath := filepath.Join(s.root, "kernels", "default")
	initrdPath := filepath.Join(s.root, "kernels", "initrd")

	return errors.Join(
		os.WriteFile(kernelPath, boot.KernelBin(), 0600),
		os.WriteFile(initrdPath, boot.InitrdBin(), 0600),
	)
}

func (s *Storage) KernelPath() string {
	return filepath.Join(s.root, "kernels", "default")
}

func (s *Storage) InitrdPath() string {
	return filepath.Join(s.root, "kernels", "initrd")
}

type RootDiskBuilder func(w *erofs.Writer) error

func (s *Storage) PutVMConfig(name string, cfg types.VM) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(s.root, "vm", fmt.Sprintf("%s.yml", name))

	return os.WriteFile(cfgPath, b, 0600)
}

func (s *Storage) CreateVMRootDisk(name string, rdBuilder RootDiskBuilder) error {
	rdPath := filepath.Join(s.root, "vm", fmt.Sprintf("%s.img", name))
	fd, err := os.OpenFile(rdPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)

	if err != nil {
		return err
	}

	now := time.Now()
	img := erofs.Create(
		fd, erofs.WithBuildTime(uint64(now.Unix()), uint32(now.Nanosecond())),
	)

	if err := rdBuilder(img); err != nil {
		return errors.Join(err, fd.Close())
	}
	if err := img.Close(); err != nil {
		return errors.Join(err, fd.Close())
	}

	return fd.Close()
}

func (s *Storage) GetVMConfig(name string) (types.VM, error) {
	cfg := new(types.VM)
	cfgPath := filepath.Join(s.root, "vm", fmt.Sprintf("%s.yml", name))
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return *cfg, err
	}
	err = yaml.Unmarshal(b, cfg)
	return *cfg, err
}

func (s *Storage) ListVMConfigs() ([]types.VM, error) {
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

func (s *Storage) GetVMRootDisk(name string) (string, error) {
	fp := filepath.Join(s.root, "vm", fmt.Sprintf("%s.img", name))
	_, err := os.Stat(fp)
	return fp, err
}

func (s *Storage) DeleteVM(name string) error {
	return errors.Join(
		removeIfExists(filepath.Join(s.root, "vm", fmt.Sprintf("%s.yml", name))),
		removeIfExists(filepath.Join(s.root, "vm", fmt.Sprintf("%s.img", name))),
	)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Storage) PutImage(key string, ref name.Reference, img v1.Image) error {
	imagePath := filepath.Join(s.root, "images", key)
	tmp, err := os.CreateTemp(filepath.Dir(imagePath), ".image-*.tar")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	if err := tarball.WriteToFile(tmpPath, ref, img); err != nil {
		return err
	}
	return os.Rename(tmpPath, imagePath)
}
func (s *Storage) GetImage(key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.root, "images", key))
}

func (s *Storage) DeleteImage(key string) error {
	return os.Remove(filepath.Join(s.root, "images", key))
}
