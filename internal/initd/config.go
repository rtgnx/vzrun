package initd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type InitConfig struct {
	EnvVars     map[string]string
	Secrets     map[string]string
	ExecCommand string
	ExecArgs    []string
}

func (tc InitConfig) Env() (envs []string) {
	envs = []string{}
	for k, v := range tc.EnvVars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	return envs
}

func (tc InitConfig) Encode() (string, error) {
	b, err := json.Marshal(tc)
	return base64.StdEncoding.EncodeToString(b), err
}

func (tc *InitConfig) Decode(s string) error {
	s = strings.TrimPrefix(s, "b64:")
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, tc)
}

func ConfigFromKernelCmdline() (string, error) {
	b, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return "", err
	}
	for _, field := range strings.Fields(string(b)) {
		if value, ok := strings.CutPrefix(field, "tc="); ok {
			return value, nil
		}
	}
	return "", fmt.Errorf("tc kernel parameter not found")
}
