package volumes

import (
	"context"
	"fmt"
	"os"

	"github.com/rtgnx/vzrun/internal/vzd/storage"
	"github.com/rtgnx/vzrun/pkg/types"
	"github.com/tmc/apple/virtualization"
	"github.com/tmc/apple/x/vzkit"
)

const VolumeKeyFmt = `%s/%s.img`

type Volume struct {
	UUID string
	types.Volume
	dev virtualization.VZStorageDeviceConfiguration
}

func New(store storage.Store) VolumeManager { return VolumeManager{store} }

type VolumeManager struct {
	store storage.Store
}

func (v VolumeManager) Create(ctx context.Context, vm string, vol types.Volume) (Volume, error) {
	newVol := Volume{Volume: vol}

	key := fmt.Sprintf(VolumeKeyFmt, vm, vol.Name)
	fp, err := v.store.Lookup(ctx, storage.KindVM, key)

	if err != nil {
		if os.IsNotExist(err) {
			fd, err := v.store.Create(ctx, storage.KindVM, key)
			if err != nil {
				return newVol, err
			}
			defer fd.Close()
			if err := fd.Truncate(int64(vol.SizeGB) << 30); err != nil {
				return newVol, err
			}
			fp, err = v.store.Lookup(ctx, storage.KindVM, key)
			if err != nil {
				return newVol, err
			}
		} else {
			return newVol, err
		}
	}

	attachment, err := vzkit.CreateDiskAttachment(fp, vol.ReadOnly)
	if err != nil {
		return newVol, err
	}
	newVol.dev = vzkit.CreateBlockDevice(attachment).VZStorageDeviceConfiguration
	return newVol, nil
}

func (v Volume) Device() virtualization.VZStorageDeviceConfiguration {
	return v.dev
}
