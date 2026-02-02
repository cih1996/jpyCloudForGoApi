package socks5

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/ghp3000/logs"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/xtaci/smux"
	"io"
	"net"
	"runtime/debug"
	"socks5/bufferpool"
	"socks5/statute"
)

const (
	chunkSize = 32 * 1024
)

type Dialer struct {
	mux  *smux.Session
	pool bufferpool.BufPool
}

func NewDialer(rw io.ReadWriteCloser, cfg *smux.Config) (*Dialer, error) {
	if rw == nil {
		return nil, errors.New("conn is nil")
	}
	mux, err := smux.Server(rw, cfg)
	if err != nil {
		return nil, err
	}
	return &Dialer{mux: mux, pool: bufferpool.NewPool(chunkSize)}, nil
}
func (s *Dialer) Start() {
	defer func() {
		if err := recover(); err != nil {
			logs.Fatal(err)
			logs.Fatal(string(debug.Stack()))
		}
	}()
	for {
		stream, err := s.mux.AcceptStream()
		if err != nil {
			s.Close()
			return
		}
		go s.handle(stream)
	}
}
func (s *Dialer) handle(stream *smux.Stream) {
	defer func() {
		if err := recover(); err != nil {
			logs.Fatal(err)
			logs.Fatal(string(debug.Stack()))
		}
		stream.Close()
	}()
	header, err := s.parserHeader(stream)
	if err != nil {
		logs.Info("stream %d,err=%s", stream.ID(), err.Error())
		return
	}
	if err = header.VerifyAddr(); err != nil {
		logs.Info("stream %d,err=%s", stream.ID(), err.Error())
		return
	}
	addr := header.AddrToString()
	if addr == "" {
		logs.Info("stream %d,addr is null", stream.ID())
		return
	}
	switch header.Type {
	case statute.ConnectTypeTCP:
		if err = s.forwardTCP(stream, header, addr); err != nil {
			logs.Info("stream %d,err=%s", stream.ID(), err.Error())
		}
	case statute.ConnectTypeUDP:
		if err = s.forwardUDP(stream, header, addr); err != nil {
			logs.Info("stream %d,err=%s", stream.ID(), err.Error())
		}
	default:
		//logs.Debug("unsupported connect type")
		return
	}
}
func (s *Dialer) Close() error {
	if s.mux == nil {
		return nil
	}
	return s.mux.Close()
}

func (s *Dialer) forwardTCP(stream *smux.Stream, header *statute.Header, addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		header.Data = []byte(err.Error())
		if err = reply(stream, header); err != nil {
			return err
		}
		return err
	}
	if err = reply(stream, header); err != nil {
		return err
	}
	buf1 := s.pool.Get()[:chunkSize]
	defer s.pool.Put(buf1)
	buf2 := s.pool.Get()[:chunkSize]
	defer s.pool.Put(buf2)
	go copyBuffer(conn, stream, buf1)
	copyBuffer(stream, conn, buf2)
	return nil
}
func (s *Dialer) forwardUDP(stream *smux.Stream, header *statute.Header, addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		header.Data = []byte(err.Error())
		_ = reply(stream, header)
		return err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		header.Data = []byte(err.Error())
		_ = reply(stream, header)
		return err
	}
	_ = reply(stream, header)
	buf1 := s.pool.Get()[:chunkSize]
	defer s.pool.Put(buf1)
	buf2 := s.pool.Get()[:chunkSize]
	defer s.pool.Put(buf2)
	go copyBuffer(conn, stream, buf1)
	copyBuffer(stream, conn, buf2)
	return nil
}
func copyBuffer(dst io.ReadWriteCloser, src io.ReadWriteCloser, buf []byte) (int64, error) {
	defer func() {
		if err := recover(); err != nil {
			logs.Fatal(err)
			logs.Fatal(string(debug.Stack()))
		}
	}()
	defer dst.Close()
	defer src.Close()
	return io.CopyBuffer(dst, src, buf)
}

func (s *Dialer) parserHeader(r io.Reader) (*statute.Header, error) {
	var length uint16
	err := binary.Read(r, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}
	buf := s.pool.Get()[:length]
	defer s.pool.Put(buf)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	if n != int(length) {
		return nil, fmt.Errorf("unexpected data length read: %d, expected: %d", n, length)
	}
	var header statute.Header
	err = msgpack.Unmarshal(buf, &header)
	if err != nil {
		return nil, err
	}
	return &header, nil
}
func reply(stream *smux.Stream, header *statute.Header) error {
	buf, err := msgpack.Marshal(header)
	if err != nil {
		return err
	}
	err = binary.Write(stream, binary.LittleEndian, uint16(len(buf)))
	if err != nil {
		return err
	}
	return binary.Write(stream, binary.LittleEndian, buf)
}
