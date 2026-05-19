# vzrun

Fast, lightweight Linux VMs on macOS, built from OCI images and powered by
Apple's `Virtualization.framework`.

`vzrun` is an experimental VM runner with a small daemon/CLI split:

- `vzd` owns VM lifecycle, storage, networking, and console sessions.
- `vz` talks to `vzd` over a local Unix socket.
- root filesystems are generated from OCI/Docker images.
- guests boot through a small custom init flow.

The goal is a Firecracker-style workflow, but native to macOS.

## Status

This project is early alpha. The current path is useful for local experiments
and VM/container runtime work, but the interface and on-disk layout may still
change.

Implemented:

- create, start, stop, restart, delete, and list VMs
- OCI image fetch and cached root filesystem generation
- Apple Virtualization NAT networking
- serial console log streaming with a bounded in-memory buffer
- interactive console attach
- custom arm64 Linux kernel and initrd build path

Not implemented yet:

- persistent volumes
- host port forwarding / service exposure
- packaged LaunchAgent install flow for `vzd`
- signed and notarized GitHub releases

## Requirements

- macOS on Apple Silicon
- Go
- Docker, used to build the custom Linux kernel
- Xcode command line tools, used by `codesign`

`vzd` must be signed with the Apple Virtualization entitlement. The Makefile
currently uses local ad-hoc signing with `virtualization.entitlements`.

## Build

```sh
make all
```

This builds:

- `bin/vz`
- `bin/vzd`
- `internal/initd/boot/Image`
- `internal/initd/boot/initrd.cpio`

## Usage

Start the daemon:

```sh
bin/vzd
```

In another shell, create and start a VM:

```sh
bin/vz create --cpu 1 --memory 1 --name busybox busybox:stable
bin/vz start -i -t busybox
```

Useful commands:

```sh
bin/vz ps
bin/vz logs busybox
bin/vz attach busybox
bin/vz stop busybox
bin/vz delete busybox
```

`vz start -i NAME` streams console output after starting the VM.

`vz start -i -t NAME` starts the VM and attaches stdin/stdout to the serial
console. `Ctrl-D` detaches.

`vz attach NAME` attaches to an existing VM console.

## Storage

By default, `vzd` stores state under:

```text
~/.vzrun
```

This includes cached OCI artifacts, generated root filesystems, VM configs,
kernel/initrd files, and the daemon socket.

## Roadmap

- persistent volumes
- service exposure and port forwarding
- Tailscale Service VIP support
- LaunchAgent install/uninstall commands
- signed release packaging
