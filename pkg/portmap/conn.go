package portmap

import (
	"time"
)

const (
	defaultTimeout = time.Second * 30
	chunkSize      = 32 * 1024
)

type ConnWithDeadline interface {
	Read(b []byte) (int, error)
	Write(b []byte) (int, error)
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Close() error
}

type timeoutConn struct {
	ConnWithDeadline
	timeout time.Duration
}

func WrapConnWithTimeout(conn ConnWithDeadline, timeout time.Duration) ConnWithDeadline {
	return &timeoutConn{ConnWithDeadline: conn, timeout: timeout}
}
func (c *timeoutConn) Read(b []byte) (int, error) {
	//_ = c.ConnWithDeadline.SetReadDeadline(time.Now().Add(c.timeout))
	n, err := c.ConnWithDeadline.Read(b)
	return n, err
}

func (c *timeoutConn) Write(b []byte) (int, error) {
	n, err := c.ConnWithDeadline.Write(b)
	return n, err
}
