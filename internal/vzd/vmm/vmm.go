//go:build darwin

package vmm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/rtgnx/vzrun/internal/vzd/vmm/console"
	"github.com/tmc/apple/virtualization"
	"github.com/tmc/apple/x/vzkit"
)

var (
	ErrVMNotFound     = errors.New("vm not found")
	ErrVMExists       = errors.New("vm exists")
	ErrUnsupportedCtl = errors.New("unsupported vm ctl")
	ErrDiskRequired   = errors.New("disk path required")
	ErrVMRunning      = errors.New("vm is running")
)

type VM struct {
	Name     string
	vm       virtualization.VZVirtualMachine
	vzConfig virtualization.VZVirtualMachineConfiguration
	console  *console.Console
	queue    *vzkit.Queue
}

type VMM struct {
	vms map[string]*VM
	mx  sync.RWMutex
}

func New() *VMM {
	return &VMM{make(map[string]*VM), sync.RWMutex{}}
}

func (vmm *VMM) vmctl(ctx context.Context, name string, state virtualization.VZVirtualMachineState) error {
	vmm.mx.RLock()
	vm, ok := vmm.vms[name]
	vmm.mx.RUnlock()
	if !ok || vm == nil || vm.queue == nil {
		return ErrVMNotFound
	}

	done := make(chan error, 1)
	currentState := vzkit.VMState(vm.queue, vm.vm)

	switch state {
	case virtualization.VZVirtualMachineStateRunning:
		if !vzkit.CanStart(vm.queue, vm.vm) {
			return fmt.Errorf("vm %q cannot start from state %s", name, state)
		}
		vzkit.StartVM(vm.queue, vm.vm, func(err error) { done <- err })
	case virtualization.VZVirtualMachineStateStopped:

		if !vzkit.CanStop(vm.queue, vm.vm) {
			return fmt.Errorf("vm %q cannot stop from state %s", name, state)
		}
		vzkit.StopVM(vm.queue, vm.vm, func(err error) { done <- err })
	default:
		return ErrUnsupportedCtl
	}

	select {
	case err := <-done:
		if err != nil {
			nextState := vzkit.VMState(vm.queue, vm.vm)
			return fmt.Errorf("vm %q %s failed from state %s to %s: %w", name, state, currentState, nextState, err)
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (vmm *VMM) Create(ctx context.Context, name string, cfg virtualization.VZVirtualMachineConfiguration) error {

	if err := vzkit.ValidateConfig(cfg); err != nil {
		return err
	}

	vmm.mx.Lock()
	defer vmm.mx.Unlock()

	if _, ok := vmm.vms[name]; ok {
		return ErrVMExists
	}

	console, err := console.New()

	if err != nil {
		return err
	}

	if err := console.VZAttachSerial(&cfg); err != nil {
		console.Close()
		return err
	}
	vmm.vms[name] = &VM{
		Name:     name,
		vzConfig: cfg,
		queue:    vzkit.NewQueue(name),
		console:  console,
	}
	vmm.vms[name].vm = vzkit.CreateVM(cfg, vmm.vms[name].queue)

	return nil
}

func (vmm *VMM) Destroy(ctx context.Context, name string) error {
	vmm.mx.Lock()
	defer vmm.mx.Unlock()

	vm, ok := vmm.vms[name]
	if !ok || vm == nil || vm.queue == nil {
		return ErrVMNotFound
	}

	state := vzkit.VMState(vm.queue, vm.vm)
	switch state {
	case virtualization.VZVirtualMachineStateStopped, virtualization.VZVirtualMachineStateError:
		if err := vm.console.Close(); err != nil {
			return err
		}
		delete(vmm.vms, name)
		return nil
	default:
		return fmt.Errorf("%w: vm %q is in state %s", ErrVMRunning, name, state)
	}

}

func (vmm *VMM) List(ctx context.Context) map[string]virtualization.VZVirtualMachineState {
	list := map[string]virtualization.VZVirtualMachineState{}

	vmm.mx.RLock()
	defer vmm.mx.RUnlock()
	for name, vm := range vmm.vms {
		if vm != nil && vm.queue != nil {
			list[name] = vzkit.VMState(vm.queue, vm.vm)
		}
	}

	return list
}

func (vmm *VMM) Exists(ctx context.Context, name string) bool {
	vmm.mx.RLock()
	defer vmm.mx.RUnlock()
	vm, ok := vmm.vms[name]
	return ok && vm != nil
}

func (vmm *VMM) Start(ctx context.Context, name string) error {
	return vmm.vmctl(ctx, name, virtualization.VZVirtualMachineStateRunning)
}

func (vmm *VMM) Stop(ctx context.Context, name string) error {
	return vmm.vmctl(ctx, name, virtualization.VZVirtualMachineStateStopped)
}

func (vmm *VMM) Logs(ctx context.Context, name string, w io.Writer) error {
	vmm.mx.RLock()
	vm, ok := vmm.vms[name]
	vmm.mx.RUnlock()

	if !ok || vm == nil {
		return ErrVMNotFound
	}

	return vm.console.Stream(ctx, w)
}

func (vmm *VMM) Attach(ctx context.Context, name string, r io.ReadCloser, w io.Writer) error {
	vmm.mx.RLock()
	vm, ok := vmm.vms[name]
	vmm.mx.RUnlock()

	if !ok || vm == nil {
		return ErrVMNotFound
	}

	return vm.console.Attach(ctx, r, w)
}
