package oci

import (
	"fmt"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/rtgnx/vzrun/pkg/types"
)

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
