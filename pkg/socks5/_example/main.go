package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/ghp3000/logs"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/xtaci/smux"
	"io"
	"log"
	"net"
	"os"
	"socks5"
	"socks5/statute"
	"time"
)

func main() {
	go testDialer()
	time.Sleep(time.Millisecond * 100)
	server()
}

func server() {
	conn, err := net.Dial("tcp", "127.0.0.1:10000")
	if err != nil {
		panic(err)
	}
	cfg := smux.DefaultConfig()
	cfg.KeepAliveInterval = 3 * time.Second
	cfg.MaxReceiveBuffer = 4 * 1024 * 1024 // 每个 stream 最多可缓冲 4MB 数据
	cfg.MaxStreamBuffer = 512 * 1024       // 每个流写缓存 512KB
	mux, err := smux.Client(conn, cfg)
	if err != nil {
		panic(err)
	}
	s5 := socks5.NewServer(
		socks5.WithLogger(socks5.NewLogger(log.New(os.Stdout, "socks5: ", log.LstdFlags))),
		socks5.WithDial(func(ctx context.Context, network, address string, addrSpec *statute.AddrSpec) (net.Conn, error) {
			fmt.Printf("%s,%s,%v\n", network, address, addrSpec)
			stream, err := mux.OpenStream()
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
			err = handshake(stream, header)
			if err != nil {
				return nil, err
			}
			return stream, nil
		}),
		socks5.WithDialAndRequest(func(ctx context.Context, network, addr string, request *socks5.Request) (net.Conn, error) {
			stream, err := mux.OpenStream()
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
			err = handshake(stream, header)
			if err != nil {
				return nil, err
			}
			return stream, nil
		}),
	)
	if err = s5.ListenAndServe("tcp", ":10800"); err != nil {
		panic(err)
	}
}
func handshake(stream *smux.Stream, header *statute.Header) error {
	buf, err := msgpack.Marshal(header)
	if err != nil {
		return nil
	}
	err = binary.Write(stream, binary.LittleEndian, uint16(len(buf)))
	if err != nil {
		return err
	}
	_, err = stream.Write(buf)
	h, err := parserHeader(stream)
	if err != nil {
		return err
	}
	if len(h.Data) > 1 {
		return errors.New(string(h.Data))
	}
	return nil
}
func parserHeader(r io.Reader) (*statute.Header, error) {
	var length uint16
	err := binary.Read(r, binary.LittleEndian, &length)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, length)
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
func testDialer() {
	logs.Info("s5 remote dialer listen at 10000")
	ln, err := net.Listen("tcp", ":10000")
	if err != nil {
		logs.Info(err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			logs.Info(err)
			return
		}
		go func() {
			logs.Info("%s in", conn.RemoteAddr().String())
			defer conn.Close()
			cfg := smux.DefaultConfig()
			cfg.KeepAliveInterval = 3 * time.Second
			cfg.MaxReceiveBuffer = 4 * 1024 * 1024 // 每个 stream 最多可缓冲 4MB 数据
			cfg.MaxStreamBuffer = 512 * 1024       // 每个流写缓存 512KB
			dialer, err := socks5.NewDialer(conn, cfg)
			if err != nil {
				logs.Info(err)
				return
			}
			dialer.Start()
			logs.Error("连接断开")
		}()
	}
}
