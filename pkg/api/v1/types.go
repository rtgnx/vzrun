//go:build darwin

package v1

import "github.com/rtgnx/vzrun/pkg/types"

type CreateVMRequest struct {
	types.VM
}

type ListVMsResponse struct {
	List map[string]string `json:"list"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
