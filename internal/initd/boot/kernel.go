package bin

import (
	"os"

	_ "embed"
)

//go:embed Image
var kernel []byte

func WriteKernel(path string) error {
	return os.WriteFile(path, kernel, 0600)
}

func KernelBin() []byte {
	return kernel
}
