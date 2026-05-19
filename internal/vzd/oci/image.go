package oci

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/erofs/go-erofs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rtgnx/vzrun/internal/vzd/storage/osfs"
	"github.com/rtgnx/vzrun/pkg/types"
)

func FetchImage(ctx context.Context, store *osfs.Storage, ref string) (string, error) {

	imageRef, err := name.ParseReference(ref)

	if err != nil {
		return "", fmt.Errorf("parse image reference: ref=%s, err=%w", ref, err)
	}

	img, err := remote.Image(
		imageRef, remote.WithContext(ctx), remote.WithPlatform(
			v1.Platform{OS: "linux", Architecture: runtime.GOARCH},
		),
	)

	if err != nil {
		return "", fmt.Errorf("fetch image reference: ref=%s, err=%w", ref, err)
	}

	size, err := img.Size()

	if err != nil {
		return "", fmt.Errorf("calculate image size: size=%d, err=%w", size, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("calculate image digest: digest=%s, err=%w", digest.String(), err)
	}

	key := digest.String()
	if err := validateCachedImage(store, key); err != nil {
		if !os.IsNotExist(err) {
			if err := store.DeleteImage(key); err != nil {
				return "", fmt.Errorf("delete invalid cached image: key=%s, err=%w", key, err)
			}
		}
		return key, store.PutImage(key, imageRef, img)
	}

	return key, nil
}

func BuildRootDisk(ctx context.Context, store *osfs.Storage, name string, ref string) (types.Exec, error) {

	key, err := FetchImage(ctx, store, ref)
	if err != nil {
		return types.Exec{}, err
	}

	img, err := tarball.Image(
		func() (io.ReadCloser, error) {
			return store.GetImage(key)
		},
		nil,
	)
	if err != nil {
		return types.Exec{}, fmt.Errorf("load image: key=%s, err=%w", key, err)
	}

	cfg, err := ExecFromImage(img)
	if err != nil {
		return types.Exec{}, err
	}

	rootfs := mutate.Extract(img)
	defer rootfs.Close()
	return cfg, store.CreateVMRootDisk(
		name, func(w *erofs.Writer) error {
			return UntarToEroFS(ctx, w, rootfs, NopProgress)
		},
	)
}

func ExecFromImage(img v1.Image) (types.Exec, error) {
	cfg, err := img.ConfigFile()
	if err != nil {
		return types.Exec{}, fmt.Errorf("read image config: %w", err)
	}

	tc := types.Exec{
		Env: imageEnv(cfg.Config.Env),
	}
	switch {
	case len(cfg.Config.Entrypoint) > 0:
		tc.Command = cfg.Config.Entrypoint[0]
		tc.Args = append(append([]string{}, cfg.Config.Entrypoint[1:]...), cfg.Config.Cmd...)
	case len(cfg.Config.Cmd) > 0:
		tc.Command = cfg.Config.Cmd[0]
		tc.Args = append([]string{}, cfg.Config.Cmd[1:]...)
	default:
		return types.Exec{}, fmt.Errorf("image has no entrypoint or cmd")
	}
	return tc, nil
}

func imageEnv(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, value := range env {
		key, val, ok := strings.Cut(value, "=")
		if ok {
			out[key] = val
		}
	}
	return out
}

func validateCachedImage(store *osfs.Storage, key string) error {
	_, err := tarball.Image(
		func() (io.ReadCloser, error) {
			return store.GetImage(key)
		},
		nil,
	)
	return err
}
