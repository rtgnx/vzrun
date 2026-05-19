.PHONY: all test clean

BIN_DIR := bin
INTERNAL_BIN_DIR := internal/initd/boot
ENTITLEMENTS := virtualization.entitlements

VZ := $(BIN_DIR)/vz
VZD := $(BIN_DIR)/vzd
KERNEL_IMAGE := $(INTERNAL_BIN_DIR)/Image
INITRD := $(INTERNAL_BIN_DIR)/initrd.cpio

all: $(VZ) $(VZD)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(KERNEL_IMAGE): Dockerfile.kernel
	docker buildx build --platform linux/arm64 -f Dockerfile.kernel --output type=local,dest=$(INTERNAL_BIN_DIR) .

$(INITRD): $(shell find cmd/initd internal/initd -name '*.go') internal/initd/boot/initrd.go go.mod go.sum
	go generate ./internal/initd/boot

$(VZ): $(shell find cmd/vz internal -name '*.go') go.mod go.sum | $(BIN_DIR)
	go build -o $(VZ) ./cmd/vz

$(VZD): $(shell find cmd/vzd internal -name '*.go') go.mod go.sum $(ENTITLEMENTS) $(INITRD) $(KERNEL_IMAGE) | $(BIN_DIR)
	go build -o $(VZD) ./cmd/vzd
	codesign --force --sign - --entitlements $(ENTITLEMENTS) $(VZD)


test:
	go test ./...

clean:
	rm -rf $(BIN_DIR)
