package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
)

type Client struct {
	client  *http.Client
	baseURL string
}

func NewClient(socketPath string) *Client {
	return &Client{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var dialer net.Dialer
					conn, err := dialer.DialContext(ctx, "unix", socketPath)
					if err != nil {
						return nil, normalizeDialError(socketPath, err)
					}
					return conn, nil
				},
			},
		},
		baseURL: "http://unix",
	}
}

func normalizeDialError(socketPath string, err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("vzd is not running: socket %s does not exist", socketPath)
	case errors.Is(err, syscall.ECONNREFUSED):
		return fmt.Errorf("vzd is not accepting connections on %s", socketPath)
	default:
		return err
	}
}

func (c *Client) Create(ctx context.Context, name string, req CreateVMRequest) error {
	return c.doJSON(ctx, http.MethodPost, VMPath(name), req, http.StatusCreated)
}

func (c *Client) Delete(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, VMPath(name), nil, http.StatusNoContent)
}

func (c *Client) Start(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, VMStartPath(name), nil, http.StatusNoContent)
}

func (c *Client) Stop(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, VMStopPath(name), nil, http.StatusNoContent)
}

func (c *Client) Restart(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPost, VMRestartPath(name), nil, http.StatusNoContent)
}

func (c *Client) List(ctx context.Context) (*ListVMsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+VMsRoute, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, unwrapTransportError(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, decodeError(res)
	}

	out := new(ListVMsResponse)
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Logs(ctx context.Context, name string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+VMLogsPath(name), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, unwrapTransportError(err)
	}
	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		return nil, decodeError(res)
	}
	return res.Body, nil
}

func (c *Client) Attach(ctx context.Context, name string, input io.Reader) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+VMAttachPath(name), input)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, unwrapTransportError(err)
	}
	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		return nil, decodeError(res)
	}
	return res.Body, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, wantStatus int) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return unwrapTransportError(err)
	}
	defer res.Body.Close()
	if res.StatusCode == wantStatus {
		return nil
	}
	return decodeError(res)
}

func unwrapTransportError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

func decodeError(res *http.Response) error {
	var apiErr ErrorResponse
	if err := json.NewDecoder(res.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
		return fmt.Errorf("http %d: %s", res.StatusCode, apiErr.Error)
	}
	return fmt.Errorf("http %d: %s", res.StatusCode, res.Status)
}
