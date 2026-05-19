//go:build darwin

package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rtgnx/vzrun/pkg/types"
	"github.com/tmc/apple/virtualization"
)

func TestLogsVMReturnsNotFoundBeforeStreaming(t *testing.T) {
	mgr := &fakeVMManager{}
	req := httptest.NewRequest(http.MethodGet, "/v1/vms/missing/logs", nil)
	req.SetPathValue("name", "missing")
	res := httptest.NewRecorder()

	logsVM(mgr).ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if mgr.logsCalled {
		t.Fatal("Logs() was called for a missing VM")
	}
}

func TestAttachVMReturnsNotFoundBeforeStreaming(t *testing.T) {
	mgr := &fakeVMManager{}
	req := httptest.NewRequest(http.MethodPost, "/v1/vms/missing/attach", nil)
	req.SetPathValue("name", "missing")
	res := httptest.NewRecorder()

	attachVM(mgr).ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if mgr.attachCalled {
		t.Fatal("Attach() was called for a missing VM")
	}
}

type fakeVMManager struct {
	logsCalled   bool
	attachCalled bool
}

func (m *fakeVMManager) CreateVM(context.Context, types.VM) error {
	return nil
}

func (m *fakeVMManager) List(context.Context) map[string]virtualization.VZVirtualMachineState {
	return nil
}

func (m *fakeVMManager) Exists(context.Context, string) bool {
	return false
}

func (m *fakeVMManager) Start(context.Context, string) error {
	return nil
}

func (m *fakeVMManager) Stop(context.Context, string) error {
	return nil
}

func (m *fakeVMManager) Restart(context.Context, string) error {
	return nil
}

func (m *fakeVMManager) Destroy(context.Context, string) error {
	return nil
}

func (m *fakeVMManager) Logs(context.Context, string, io.Writer) error {
	m.logsCalled = true
	return nil
}

func (m *fakeVMManager) Attach(context.Context, string, io.ReadCloser, io.Writer) error {
	m.attachCalled = true
	return nil
}
