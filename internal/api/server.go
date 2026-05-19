//go:build darwin

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"

	apiv1 "github.com/rtgnx/vzrun/internal/api/v1"
	"github.com/rtgnx/vzrun/internal/vzd/vmm"
	"github.com/rtgnx/vzrun/pkg/types"
	"github.com/tmc/apple/virtualization"
)

type VMManager interface {
	CreateVM(context.Context, types.VM) error
	List(context.Context) map[string]virtualization.VZVirtualMachineState
	Exists(context.Context, string) bool
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
	Destroy(context.Context, string) error
	Logs(context.Context, string, io.Writer) error
	Attach(context.Context, string, io.ReadCloser, io.Writer) error
}

func Start(ctx context.Context, socketPath string, mgr VMManager) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0700); err != nil {
		return err
	}

	listener, err := listenUnix(socketPath)
	if err != nil {
		return err
	}
	defer listener.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+apiv1.VMsRoute, listVMs(mgr))
	mux.HandleFunc("POST "+apiv1.VMRoute, createVM(mgr))
	mux.HandleFunc("DELETE "+apiv1.VMRoute, deleteVM(mgr))
	mux.HandleFunc("POST "+apiv1.VMStartRoute, controlVM(mgr.Start))
	mux.HandleFunc("POST "+apiv1.VMStopRoute, controlVM(mgr.Stop))
	mux.HandleFunc("POST "+apiv1.VMRestartRoute, controlVM(mgr.Restart))
	mux.HandleFunc("GET "+apiv1.VMLogsRoute, logsVM(mgr))
	mux.HandleFunc("POST "+apiv1.VMAttachRoute, attachVM(mgr))

	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func listenUnix(socketPath string) (net.Listener, error) {
	info, err := os.Lstat(socketPath)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("socket path %s already exists and is not a socket", socketPath)
		}

		conn, dialErr := net.Dial("unix", socketPath)
		if dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("vzd is already running on %s", socketPath)
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, dialErr
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, err
		}
	case os.IsNotExist(err):
	default:
		return nil, err
	}

	return net.Listen("unix", socketPath)
}

func createVM(mgr VMManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := requireName(w, r)
		if !ok {
			return
		}

		req := new(apiv1.CreateVMRequest)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		req.Name = name
		if req.CPUs <= 0 || req.MemoryGB <= 0 || req.Image == "" {
			writeError(w, http.StatusBadRequest, errors.New("cpus, memoryGB, and image are required"))
			return
		}

		if err := mgr.CreateVM(r.Context(), req.VM); err != nil {
			writeVMMError(w, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}

func deleteVM(mgr VMManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := requireName(w, r)
		if !ok {
			return
		}
		if err := mgr.Destroy(r.Context(), name); err != nil {
			writeVMMError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listVMs(mgr VMManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		states := mgr.List(r.Context())
		res := apiv1.ListVMsResponse{List: make(map[string]string, len(states))}
		for name, state := range states {
			res.List[name] = state.String()
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func controlVM(fn func(context.Context, string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := requireName(w, r)
		if !ok {
			return
		}
		if err := fn(r.Context(), name); err != nil {
			writeVMMError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func logsVM(mgr VMManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := requireName(w, r)
		if !ok {
			return
		}
		if !mgr.Exists(r.Context(), name) {
			writeVMMError(w, vmm.ErrVMNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		if err := mgr.Logs(r.Context(), name, flushWriter{w: w}); err != nil && !errors.Is(err, context.Canceled) {
			return
		}
	}
}

func attachVM(mgr VMManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, ok := requireName(w, r)
		if !ok {
			return
		}
		if !mgr.Exists(r.Context(), name) {
			writeVMMError(w, vmm.ErrVMNotFound)
			return
		}
		if err := http.NewResponseController(w).EnableFullDuplex(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		if err := mgr.Attach(r.Context(), name, r.Body, flushWriter{w: w}); err != nil && !errors.Is(err, context.Canceled) {
			return
		}
	}
}

type flushWriter struct {
	w http.ResponseWriter
}

func (w flushWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if flusher, ok := w.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}

func requireName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New("vm name required"))
		return "", false
	}
	return name, true
}

func writeVMMError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vmm.ErrVMNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, vmm.ErrVMExists), errors.Is(err, vmm.ErrVMRunning):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, vmm.ErrUnsupportedCtl):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, apiv1.ErrorResponse{Error: err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
