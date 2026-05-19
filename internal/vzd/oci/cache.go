package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/rtgnx/vzrun/internal/vzd/storage"
	"github.com/rtgnx/vzrun/pkg/types"
)

type CachedImage struct {
	Key   string
	Ref   name.Reference
	Exec  types.Exec
	store storage.Store
}

type ImageCache struct {
	store storage.Store
}

func NewImageCache(store storage.Store) ImageCache { return ImageCache{store} }

func (c ImageCache) Cache(ctx context.Context, ref name.Reference) (CachedImage, error) {
	img, err := remote.Image(
		ref, remote.WithContext(ctx), remote.WithPlatform(
			v1.Platform{OS: "linux", Architecture: runtime.GOARCH},
		),
	)

	cachedImage := CachedImage{Ref: ref, store: c.store}

	if err != nil {
		return cachedImage, fmt.Errorf("fetch image reference: ref=%s, err=%w", ref, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return cachedImage, fmt.Errorf("calculate image digest: digest=%s, err=%w", digest.String(), err)
	}

	cachedImage.Key = digest.String()

	_, err = c.store.Lookup(ctx, storage.KindImage, digest.String())
	if err == nil {
		ociImage, err := c.loadCachedImage(ctx, cachedImage)
		if err == nil {
			exec, err := ExecFromImage(ociImage)
			if err != nil {
				return cachedImage, err
			}
			cachedImage.Exec = exec
			return cachedImage, nil
		}
		if err := c.store.Remove(ctx, storage.KindImage, digest.String()); err != nil {
			return cachedImage, err
		}
		err = os.ErrNotExist
	}

	if !os.IsNotExist(err) {
		return cachedImage, err
	}

	wc, err := c.store.Create(ctx, storage.KindImage, digest.String())

	if err != nil {
		return cachedImage, err
	}

	if err := tarball.Write(ref, img, wc); err != nil {
		err := errors.Join(err, wc.Close(), c.store.Remove(ctx, storage.KindImage, digest.String()))
		return cachedImage, err
	}
	if err := wc.Close(); err != nil {
		err := errors.Join(err, c.store.Remove(ctx, storage.KindImage, digest.String()))
		return cachedImage, err
	}

	exec, err := ExecFromImage(img)
	if err != nil {
		return cachedImage, err
	}
	cachedImage.Exec = exec

	return cachedImage, nil

}

func (c CachedImage) Open(ctx context.Context) (io.ReadCloser, error) {
	return c.store.Open(ctx, storage.KindImage, c.Key)
}

func (c ImageCache) loadCachedImage(ctx context.Context, img CachedImage) (v1.Image, error) {
	imgTar, err := tarball.Image(
		func() (io.ReadCloser, error) {
			return img.Open(ctx)
		},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("load cached image: key=%s, err=%w", img.Key, err)
	}
	return imgTar, nil
}
