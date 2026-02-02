package portmap

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"socks5"
	"socks5/bufferpool"
	"socks5/statute"
	"time"

	"github.com/ghp3000/logs"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/xtaci/smux"
)

type Forwarder struct {
	mode     int
	proto    string
	port     int
	conn     io.ReadWriteCloser
	mux      *smux.Session
	listener net.Listener
	pool     bufferpool.BufPool
}

// NewForwarder
// mode: 0=连接复用的端口映射,1=无连接复用的端口映射,2=socks5代理
func NewForwarder(conn io.ReadWriteCloser, mode int, proto string, srcPort int) (*Forwarder, error) {
	if proto != "tcp" && proto != "udp" {
		return nil, fmt.Errorf("unsupported protocol: %s", proto)
	}
	if mode == 0 || mode == 2 {
		cfg := smux.DefaultConfig()
		cfg.KeepAliveInterval = 3 * time.Second
		cfg.MaxReceiveBuffer = 4 * 1024 * 1024 // 每个 stream 最多可缓冲 4MB 数据
		cfg.MaxStreamBuffer = 512 * 1024       // 每个流写缓存 512KB
		mux, err := smux.Client(conn, cfg)
		if err != nil {
			return nil, err
		}
		return &Forwarder{mode: mode, proto: proto, port: srcPort, mux: mux, pool: bufferpool.NewPool(32 * 1024)}, nil
	}
	return &Forwarder{mode: mode, proto: proto, port: srcPort, mux: nil, pool: bufferpool.NewPool(32 * 1024)}, nil
}
func (s *Forwarder) Start() error {
	defer s.Close()
	switch s.mode {
	case 0:
		return s.startMuxPortMap()
	case 2:
		return s.startProxy()
	default:
		return fmt.Errorf("forwarder: invalid mode: %d", s.mode)
	}
}
func (s *Forwarder) startMuxPortMap() error {
	var err error
	if s.proto == "tcp" {
		s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
		if err != nil {
			return err
		}
		logs.Info("port map local port:%d", s.port)
		go s.monitor()
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				return err
			}
			stream, err := s.mux.OpenStream()
			if err != nil {
				logs.Error("port map open stream port=%d,err=%s", s.port, err.Error())
				return err
			}
			logs.Info("port map stream opened,port=%d,id=%d", s.port, stream.ID())
			go s.handleForward(stream, WrapConnWithTimeout(conn, defaultTimeout))
		}
	}
	return nil
}
func (s *Forwarder) monitor() {
	<-s.mux.CloseChan()
	s.Close()
}
func (s *Forwarder) handleForward(stream *smux.Stream, conn ConnWithDeadline) {
	defer conn.Close()
	defer stream.Close()
	out := WrapConnWithTimeout(stream, defaultTimeout)

	go func() {
		buf1 := s.pool.Get()[:chunkSize]
		defer s.pool.Put(buf1)
		copyBuffer(out, conn, buf1)
		stream.Close()
	}()
	buf2 := s.pool.Get()[:chunkSize]
	defer s.pool.Put(buf2)
	copyBuffer(conn, out, buf2)
	stream.Close()
	logs.Info("port map stream closed,port=%d,id=%d", s.port, stream.ID())
}
func (s *Forwarder) Close() error {
	if s.mux != nil {
		s.mux.Close()
	}
	if s.conn != nil {
		return s.conn.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	return nil
}

func (s *Forwarder) startProxy() error {
	var err error
	go s.monitor()
	s5 := socks5.NewServer(
		//socks5.WithLogger(socks5.NewLogger(log.New(os.Stdout, "socks5: ", log.LstdFlags))),
		socks5.WithDial(func(ctx context.Context, network, address string, addrSpec *statute.AddrSpec) (net.Conn, error) {
			//fmt.Printf("%s,%s,%v\n", network, address, addrSpec)
			stream, err := s.mux.OpenStream()
			if err != nil {
				return nil, err
			}
			var connectTyp uint8
			if network == "tcp" {
				connectTyp = statute.ConnectTypeTCP
			} else {
				connectTyp = statute.ConnectTypeUDP
			}
			header := &statute.Header{
				Type:    connectTyp,
				Version: statute.VersionSocks5,
				ATYP:    addrSpec.AddrType,
				Port:    uint16(addrSpec.Port),
			}
			switch addrSpec.AddrType {
			case statute.ATYPIPv4:
				header.Addr = addrSpec.IP.To4()
			case statute.ATYPIPv6:
				header.Addr = addrSpec.IP.To16()
			case statute.ATYPDomain:
				header.Addr = []byte(addrSpec.FQDN)
			default:
				return nil, errors.New("unexpected addr type")
			}
			err = s.streamHandshake(stream, header)
			if err != nil {
				return nil, err
			}
			return stream, nil
		}),
		socks5.WithDialAndRequest(func(ctx context.Context, network, addr string, request *socks5.Request) (net.Conn, error) {
			stream, err := s.mux.OpenStream()
			if err != nil {
				return nil, err
			}
			var connectTyp uint8
			if network == "tcp" {
				connectTyp = statute.ConnectTypeTCP
			} else {
				connectTyp = statute.ConnectTypeUDP
			}
			header := &statute.Header{
				Type:    connectTyp,
				Version: statute.VersionSocks5,
				ATYP:    request.DestAddr.AddrType,
				Port:    uint16(request.DestAddr.Port),
			}
			switch request.DestAddr.AddrType {
			case statute.ATYPIPv4:
				header.Addr = request.DestAddr.IP.To4()
			case statute.ATYPIPv6:
				header.Addr = request.DestAddr.IP.To16()
			case statute.ATYPDomain:
				header.Addr = []byte(request.DestAddr.FQDN)
			default:
				return nil, errors.New("unexpected addr type")
			}
			err = s.streamHandshake(stream, header)
			if err != nil {
				return nil, err
			}
			return stream, nil
		}),
	)
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}
	return s5.Serve(s.listener)
}
func (s *Forwarder) streamHandshake(stream *smux.Stream, header *statute.Header) error {
	buf, err := msgpack.Marshal(header)
	if err != nil {
		return nil
	}
	err = binary.Write(stream, binary.LittleEndian, uint16(len(buf)))
	if err != nil {
		return err
	}
	_, err = stream.Write(buf)
	h, err := s.parserHeader(stream)
	if err != nil {
		return err
	}
	if len(h.Data) > 1 {
		return errors.New(string(h.Data))
	}
	return nil
}
func (s *Forwarder) parserHeader(r io.Reader) (*statute.Header, error) {
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
