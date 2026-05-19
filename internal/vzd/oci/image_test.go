package oci

import (
	"reflect"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/rtgnx/vzrun/pkg/types"
)

func TestExecFromImage(t *testing.T) {
	tests := []struct {
		name    string
		config  v1.Config
		want    types.Exec
		wantErr bool
	}{
		{
			name: "entrypoint only",
			config: v1.Config{
				Entrypoint: []string{"/bin/server", "--foreground"},
				Env:        []string{"A=1", "BROKEN", "B=two=parts"},
			},
			want: types.Exec{
				Command: "/bin/server",
				Args:    []string{"--foreground"},
				Env:     map[string]string{"A": "1", "B": "two=parts"},
			},
		},
		{
			name: "cmd only",
			config: v1.Config{
				Cmd: []string{"/bin/sh", "-c", "echo hi"},
				Env: []string{"PATH=/bin"},
			},
			want: types.Exec{
				Command: "/bin/sh",
				Args:    []string{"-c", "echo hi"},
				Env:     map[string]string{"PATH": "/bin"},
			},
		},
		{
			name: "entrypoint and cmd",
			config: v1.Config{
				Entrypoint: []string{"/entrypoint"},
				Cmd:        []string{"serve", "--port", "8080"},
			},
			want: types.Exec{
				Command: "/entrypoint",
				Args:    []string{"serve", "--port", "8080"},
				Env:     map[string]string{},
			},
		},
		{
			name:    "missing command",
			config:  v1.Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
				Architecture: "arm64",
				OS:           "linux",
				Config:       tt.config,
			})
			if err != nil {
				t.Fatalf("build test image config error = %v", err)
			}

			got, err := ExecFromImage(img)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ExecFromImage() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ExecFromImage() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExecFromImage() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
