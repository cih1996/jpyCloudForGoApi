package main

import (
	"encoding/hex"
	"fmt"
	"github.com/txthinking/socks5"
	"log"
	"net"
	"time"
)

func main() {
	go udpServer()
	socks5client()
}
func socks5client() {
	c, err := socks5.NewClient("192.168.2.145:10800", "", "", 0, 120)
	if err != nil {
		log.Println(err)
		return
	}
	conn, err := c.Dial("udp", "192.168.2.209:28080") //"192.168.2.191:28080"
	if err != nil {
		log.Println(err)
		return
	}
	b, err := hex.DecodeString("0001010000010000000000000a74787468696e6b696e6703636f6d0000010001")
	if err != nil {
		log.Println(err)
		return
	}
	var i int
	for {
		i++
		if _, err := conn.Write(b); err != nil {
			log.Println(err)
			return
		}
		bb := make([]byte, 2048)
		n, err := conn.Read(bb)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println(i, n, bb[:n])
		time.Sleep(1 * time.Second)
		//m := &dns.Msg{}
		//if err := m.Unpack(b[0:n]); err != nil {
		//	log.Println(err)
		//	return
		//}
		//log.Println("解析结果:", m.String())
	}
}
func udpServer() {
	addr, err := net.ResolveUDPAddr("udp", ":28080")
	if err != nil {
		log.Println(err)
		return
	}
	udp, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Println(err)
		return
	}
	defer udp.Close()
	buf := make([]byte, 2048)
	for {
		n, ar, err := udp.ReadFrom(buf)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println("udp server,", udp.RemoteAddr(), buf[:n])
		udp.WriteTo(buf[:n], ar)
	}
}
