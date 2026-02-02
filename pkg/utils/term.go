package utils

import (
	"bufio"
	"errors"
	"github.com/creack/pty"
	"io"
	"os"
	"os/exec"
	"runtime"
)

type Terminal struct {
	cmd *exec.Cmd
	pty *os.File
	rw  io.ReadWriter
}

func NewTerminal(rw io.ReadWriter, size *pty.Winsize) (*Terminal, error) {
	if rw == nil {
		return nil, errors.New("NewTerminal: rw == nil")
	}
	s := &Terminal{
		rw: rw,
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "cmd.exe"
		} else if runtime.GOOS == "linux" {
			shell = "bash"
		} else if runtime.GOOS == "android" {
			shell = "sh"
		} else if runtime.GOOS == "darwin" {
			shell = "bash"
		}
	}
	s.cmd = exec.Command(shell)
	var err error
	s.pty, err = pty.StartWithSize(s.cmd, size)
	if err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Terminal) Resize(size *pty.Winsize) error {
	if s.pty == nil {
		return errors.New("pty is closed")
	}
	return pty.Setsize(s.pty, size)
}
func (s *Terminal) Write(v []byte) error {
	if s.pty != nil {
		_, err := s.pty.Write(v)
		return err
	}
	s.Close()
	return errors.New("pty is closed")
}
func (s *Terminal) handle() {
	defer s.Close()
	reader := bufio.NewReader(s.pty)
	var n int
	var err error
	buf := make([]byte, 10240)
	for {
		n, err = reader.Read(buf)
		if err != nil {
			//logs.Debug("read data from pty err,err=[%s]", err.Error())
			return
		}
		if _, err = s.rw.Write(buf[:n]); err != nil {
			return
		}
	}
}
func (s *Terminal) Close() {
	defer func() {
		if err := recover(); err != nil {
		}
	}()
	if s.pty != nil {
		s.pty.Close()
		s.pty = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
}
