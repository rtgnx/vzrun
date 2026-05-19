//go:build darwin

package vzd

import (
	"context"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/rtgnx/vzrun/internal/initd"
	"github.com/rtgnx/vzrun/internal/vzd/oci"
	"github.com/rtgnx/vzrun/internal/vzd/storage"
	"github.com/rtgnx/vzrun/internal/vzd/storage/local"
	"github.com/rtgnx/vzrun/internal/vzd/vmm"
	"github.com/rtgnx/vzrun/internal/vzd/volumes"
	"github.com/rtgnx/vzrun/pkg/types"
	"github.com/tmc/apple/virtualization"
	"github.com/tmc/apple/x/vzkit"
)

const defaultInit = "/bin/sh"

type VZD struct {
	store  *local.Local
	vmm    *vmm.VMM
	volman volumes.VolumeManager
}

func New(dataDir string) (*VZD, error) {
	store, err := local.New(dataDir)
	if err != nil {
		return nil, err
	}
	vz := &VZD{store: store, vmm: vmm.New(), volman: volumes.New(store)}
	if err := vz.restoreVMs(context.Background()); err != nil {
		return nil, err
	}
	return vz, nil
}

func (vz *VZD) CreateVM(ctx context.Context, cfg types.VM) (err error) {
	ref, err := name.ParseReference(cfg.Image)
	if err != nil {
		return err
	}
	img, err := oci.NewImageCache(vz.store).Cache(ctx, ref)
	if err != nil {
		return err
	}
	if err := oci.NewRootDiskBuilder(vz.store).NewRootDisk(ctx, img, cfg.Name); err != nil {
		return err
	}

	if cfg.Exec.Command == "" {
		cfg.Exec = img.Exec
	}
	if err := vz.createRuntimeVM(ctx, cfg); err != nil {
		return err
	}
	return vz.store.PutVMConfig(cfg.Name, cfg)
}

func (vz *VZD) restoreVMs(ctx context.Context) error {
	configs, err := vz.store.ListVMConfigs()
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if err := vz.createRuntimeVM(ctx, cfg); err != nil {
			return err
		}
	}
	return nil
}

func (vz *VZD) createRuntimeVM(ctx context.Context, cfg types.VM) (err error) {
	linuxConfig := linuxConfigFromVM(cfg)
	linuxConfig.DiskPath, err = vz.store.Lookup(ctx, storage.KindVM, storage.VMRootDiskKey(cfg.Name))
	linuxConfig.KernelPath = vz.store.KernelPath()
	linuxConfig.InitrdPath = vz.store.InitrdPath()

	if err != nil {
		return err
	}

	vzConfig, err := vzkit.BuildLinuxVMConfig(*linuxConfig)
	if err != nil {
		return err
	}

	for _, vol := range cfg.Volumes {
		nv, err := vz.volman.Create(ctx, cfg.Name, vol)

		if err != nil {
			return err
		}
		vzConfig.SetStorageDevices(
			append(vzConfig.StorageDevices(), nv.Device()),
		)
	}

	if err := vz.vmm.Create(ctx, cfg.Name, vzConfig); err != nil {
		return err
	}
	return nil
}

func linuxConfigFromVM(vm types.VM) *vzkit.LinuxVMConfig {
	tc := initConfigFromExec(vm.Exec)
	if tc.ExecCommand == "" {
		tc.ExecCommand = defaultInit
	}
	b64, _ := tc.Encode()

	return &vzkit.LinuxVMConfig{
		CPUs:                 uint(vm.CPUs),
		MemoryGB:             uint64(vm.MemoryGB),
		Headless:             true,
		Audio:                nil,
		CmdLine:              "console=tty0 console=hvc0 rdinit=/init tc=b64:" + b64,
		NestedVirtualization: new(false),
		Network: vzkit.NetworkConfig{
			Mode: vzkit.NetworkModeNAT,
		},
		Volumes: []vzkit.VolumeMount{},
	}
}

func initConfigFromExec(exec types.Exec) initd.InitConfig {
	return initd.InitConfig{
		EnvVars:     exec.Env,
		ExecCommand: exec.Command,
		ExecArgs:    exec.Args,
	}
}

func (vz *VZD) List(ctx context.Context) map[string]virtualization.VZVirtualMachineState {
	return vz.vmm.List(ctx)
}

func (vz *VZD) Exists(ctx context.Context, name string) bool {
	return vz.vmm.Exists(ctx, name)
}

func (vz *VZD) Stop(ctx context.Context, name string) error {
	return vz.vmm.Stop(ctx, name)
}

func (vz *VZD) Start(ctx context.Context, name string) error {
	return vz.vmm.Start(ctx, name)
}

func (vz *VZD) Restart(ctx context.Context, name string) error {
	if err := vz.vmm.Stop(ctx, name); err != nil {
		return err
	}
	return vz.vmm.Start(ctx, name)
}

func (vz *VZD) Destroy(ctx context.Context, name string) error {
	if err := vz.vmm.Destroy(ctx, name); err != nil {
		return err
	}
	return vz.store.DeleteVM(name)
}

func (vz *VZD) Logs(ctx context.Context, name string, w io.Writer) error {
	return vz.vmm.Logs(ctx, name, w)
}

func (vz *VZD) Attach(ctx context.Context, name string, r io.ReadCloser, w io.Writer) error {
	return vz.vmm.Attach(ctx, name, r, w)
}
