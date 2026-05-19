//go:build darwin

package console

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/tmc/apple/virtualization"
	"github.com/tmc/apple/x/vzkit"
)

type Console struct {
	guestRead  *os.File
	guestWrite *os.File

	input    *os.File
	output   *os.File
	attached bool
	history  []byte
	subs     map[chan []byte]struct{}

	mx sync.RWMutex
}

const logBufferSize = 1 << 20

func New() (*Console, error) {
	guestR, hostW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	hostR, guestW, err := os.Pipe()
	if err != nil {
		guestR.Close()
		hostW.Close()
		return nil, err
	}

	return &Console{
		guestRead:  guestR,
		guestWrite: guestW,
		input:      hostW,
		output:     hostR,
		attached:   false,
		subs:       make(map[chan []byte]struct{}),
		mx:         sync.RWMutex{},
	}, nil
}

func (c *Console) GuestReadFD() uintptr {
	return c.guestRead.Fd()
}

func (c *Console) GuestWriteFD() uintptr {
	return c.guestWrite.Fd()
}

func (c *Console) Close() error {
	return errors.Join(
		c.guestRead.Close(),
		c.guestWrite.Close(),
		c.input.Close(),
		c.output.Close(),
	)
}

func (c *Console) Writer() io.Writer {
	return c.input
}

func (c *Console) VZAttachSerial(cfg *virtualization.VZVirtualMachineConfiguration) error {
	serial, err := vzkit.CreateSerialConsole(
		int(c.GuestReadFD()), int(c.GuestWriteFD()),
	)

	if err != nil {
		return err
	}

	cfg.SetSerialPorts([]virtualization.VZSerialPortConfiguration{
		serial.VZSerialPortConfiguration,
	})
	go c.pump()
	return nil
}

func (c *Console) Attach(ctx context.Context, r io.ReadCloser, w io.Writer) error {
	c.mx.Lock()
	if c.attached {
		c.mx.Unlock()
		return errors.New("console already attached")
	}
	c.attached = true
	c.mx.Unlock()
	defer func() {
		c.mx.Lock()
		c.attached = false
		c.mx.Unlock()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer r.Close()

	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(c.input, r)
		done <- err
	}()
	go func() {
		done <- c.Stream(ctx, w)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (c *Console) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := c.output.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			c.broadcast(chunk)
		}
		if err != nil {
			c.closeSubscribers()
			return
		}
	}
}

func (c *Console) Stream(ctx context.Context, w io.Writer) error {
	ch, history := c.subscribe()
	defer c.unsubscribe(ch)

	if len(history) > 0 {
		if _, err := w.Write(history); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p, ok := <-ch:
			if !ok {
				return io.EOF
			}
			if _, err := w.Write(p); err != nil {
				return err
			}
		}
	}
}

func (c *Console) subscribe() (chan []byte, []byte) {
	ch := make(chan []byte, 16)
	c.mx.Lock()
	c.subs[ch] = struct{}{}
	history := append([]byte(nil), c.history...)
	c.mx.Unlock()
	return ch, history
}

func (c *Console) unsubscribe(ch chan []byte) {
	c.mx.Lock()
	if _, ok := c.subs[ch]; ok {
		delete(c.subs, ch)
		close(ch)
	}
	c.mx.Unlock()
}

func (c *Console) broadcast(chunk []byte) {
	c.mx.Lock()
	defer c.mx.Unlock()
	c.appendHistory(chunk)
	for ch := range c.subs {
		select {
		case ch <- chunk:
		default:
		}
	}
}

func (c *Console) appendHistory(chunk []byte) {
	switch {
	case len(chunk) >= logBufferSize:
		c.history = append(c.history[:0], chunk[len(chunk)-logBufferSize:]...)
	case len(c.history)+len(chunk) <= logBufferSize:
		c.history = append(c.history, chunk...)
	default:
		keep := logBufferSize - len(chunk)
		c.history = append(c.history[:0], c.history[len(c.history)-keep:]...)
		c.history = append(c.history, chunk...)
	}
}

func (c *Console) closeSubscribers() {
	c.mx.Lock()
	defer c.mx.Unlock()
	for ch := range c.subs {
		close(ch)
		delete(c.subs, ch)
	}
}
