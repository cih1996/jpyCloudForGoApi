package streamRTC

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/pion/webrtc/v4"
	"sync"
)

// DataChannel 自定义数据通道，增加拆包组包逻辑
type DataChannel struct {
	channel  *webrtc.DataChannel
	length   uint32
	received uint32
	data     []byte
	lock     sync.Mutex
	doLock   sync.Mutex
}

// RawDataChannel 获取原始的数据通道。备用。请勿使用原始通道发送数据。
func (s *DataChannel) RawDataChannel() *webrtc.DataChannel {
	return s.channel
}

// Send 发送二进制数据。内置拆包发送逻辑
func (s *DataChannel) Send(buf []byte) (err error) {
	defer func() { recover() }()

	if s.channel == nil {
		return errors.New("data channel is nil")
	}
	length := len(buf)
	if length > maxPackSize {
		return fmt.Errorf("send data too large,%d > %d", length, maxPackSize)
	}
	pkt := bufferPool.Get()
	defer pkt.Put()
	rw := bytes.NewBuffer(pkt.Buff[:0])
	if err = binary.Write(rw, binary.LittleEndian, uint32(length)); err != nil {
		return err
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	if length+4 <= _maxPackSize {
		// 长度+内容,小于64k,直接组装发送
		rw.Write(buf)
		return s.channel.Send(rw.Bytes())
	} else {
		//长度+内容大于64k,先组装发送一部分,剩下的直接用buf发,少拷贝一次数据
		rw.Write(buf[:_maxPackSize-4])
		if err = s.channel.Send(rw.Bytes()); err != nil {
			return err
		}
	}
	//发送剩下的部分
	for i := _maxPackSize - 4; i < length; i += _maxPackSize {
		if _maxPackSize >= (length - i) {
			if err = s.channel.Send(buf[i:length]); err != nil {
				return err
			}
		} else {
			if err = s.channel.Send(buf[i : i+_maxPackSize]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *DataChannel) SendRaw(buf []byte) (err error) {
	defer func() { recover() }()
	if s.channel == nil {
		return errors.New("data channel is nil")
	}
	length := len(buf)
	if length > maxPackSize {
		return fmt.Errorf("send data too large,%d > %d", length, maxPackSize)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	for i := 0; i < length; i += _maxPackSize {
		if _maxPackSize > (length - i) {
			if err = s.channel.Send(buf[i:length]); err != nil {
				return err
			}
		} else {
			if err = s.channel.Send(buf[i : i+_maxPackSize]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *DataChannel) SendText(str string) error {
	if s.channel == nil {
		return errors.New("data channel is nil")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.channel.SendText(str)
}
func (s *DataChannel) Label() string {
	if s.channel == nil {
		return ""
	}
	return s.channel.Label()
}

// Do 收包到缓冲区，数据完整后执行回调函数
func (s *DataChannel) Do(c *Client, buf []byte, f OnDataChannelCallback) error {
	s.doLock.Lock()
	defer s.doLock.Unlock()
	if s.length == 0 { //新的一轮接收
		if len(buf) < 4 {
			return errors.New("data length error")
		}
		r := bytes.NewReader(buf)
		if err := binary.Read(r, binary.LittleEndian, &s.length); err != nil {
			return err
		}
		if s.length > maxPackSize {
			s.length = 0
			s.received = 0
			s.data = nil
			return fmt.Errorf("receive data length too large,%d > %d", s.length, maxPackSize)
		}
		s.received = uint32(len(buf) - 4)
		s.data = buf[4:]
	} else { //继续接收
		s.data = append(s.data, buf...)
		s.received += uint32(len(buf))
	}
	if s.received > s.length { //组包错误，收到的数据居然大于数据总大小
		s.length = 0
		s.received = 0
		s.data = nil
		return errors.New("received data length is greater than total length")
	}
	if s.received == s.length { //数据接收完整，抛回调
		if f == nil {
			return errors.New("on message callback is nil")
		}
		dst := s.data
		f(c, *s.channel.ID(), s.Label(), &webrtc.DataChannelMessage{
			IsString: false,
			Data:     dst,
		})
		//清理，准备下一次接收
		s.length = 0
		s.received = 0
		s.data = nil
	}
	return nil
}
func (s *DataChannel) Close() {
	if s.channel == nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.channel.Close()
	s.length = 0
	s.received = 0
	s.data = nil
}
