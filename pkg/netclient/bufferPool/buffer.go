package bufferPool

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/vmihailenco/msgpack/v5"
	"io"
	"reflect"
	"sync/atomic"
)

const MaxPacketSize = 10 * 1024 * 1024

type Packet struct {
	refCounter atomic.Int32
	Length     uint32
	Buff       []byte // |type|header len|header|content|
}

// ReadFromReader 按照规则从io.reader里读取数据填充Packet
func (s *Packet) ReadFromReader(r io.Reader) error {
	if r == nil {
		return errors.New("reader is nil")
	}
	if err := binary.Read(r, binary.LittleEndian, &s.Length); err != nil {
		return fmt.Errorf("read packet length err,%s", err.Error())
	}
	if s.Length > MaxPacketSize {
		//all, _ := io.ReadAll(r)
		//logs.Fatal(hex.Dump(all))
		return fmt.Errorf("packet too large %d>%d", s.Length, MaxPacketSize)
	}
	//if padding := int(s.Length) - cap(s.Buff); padding > 0 {
	//	s.Buff = make([]byte, 0, s.Length)
	//}
	//s.Buff = s.Buff[:s.Length]
	s.Resize(int(s.Length))
	if n, err := io.ReadFull(r, s.Buff); err != nil {
		return fmt.Errorf("read packet payload err,%s", err.Error())
	} else if n != int(s.Length) {
		return fmt.Errorf("unexpected data length read: %d, expected: %d", n, s.Length)
	}
	return nil
}

// ReadFromReader2 按照规则从io.reader里读取数据填充Packet
func (s *Packet) ReadFromReader2(r io.Reader) error {
	if r == nil {
		return errors.New("reader is nil")
	}
	size := 32767
	i := 1
	for {
		if padding := i*size - cap(s.Buff); padding > 0 {
			s.Buff = append(s.Buff[:cap(s.Buff)], make([]byte, padding)...)
		}
		n, err := r.Read(s.Buff[len(s.Buff):cap(s.Buff)])
		s.Length += uint32(n)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return err
		}
	}
}

// Ref 跨goroutine引用必须使用此函数，否则可能因为提前放回缓存池回收而导致数据意外缺失
func (s *Packet) Ref() *Packet {
	s.refCounter.Add(1)
	return s
}

/*
Put 每个goroutine使用结束必须Put，否则缓存池不回收导致内存持续增长

	例子： go func(p *Packet) {
			defer p.Put()
			fmt.Println(p.Buff[:p.Length])
		}(p)
*/
func (s *Packet) Put() {
	i := s.refCounter.Add(-1)
	if i == 0 {
		s.Length = 0
		s.refCounter.Store(0)
		s.Buff = s.Buff[:0]
		Buffer.put(s)
	}
}
func (s *Packet) Type() uint8 {
	if len(s.Buff) > 0 {
		return s.Buff[0]
	}
	return 0
}
func (s *Packet) ReplaceType(typ uint8) {
	if len(s.Buff) > 0 {
		s.Buff[0] = typ
	}
}

func (s *Packet) HeaderLen() uint8 {
	if len(s.Buff) > 1 {
		return s.Buff[1]
	}
	return 0
}

// HeaderRaw 头内容的原始字节
func (s *Packet) HeaderRaw() []byte {
	if headerL := s.HeaderLen(); headerL == 0 {
		return nil
	} else {
		return s.Buff[2 : 2+int(headerL)]
	}
}

// UnmarshalHeader 将头的[]byte解析为指定的变量,注意v必须传入指针.
func (s *Packet) UnmarshalHeader(v interface{}) error {
	if headerL := s.HeaderLen(); headerL == 0 {
		return nil
	} else {
		r := bytes.NewReader(s.Buff[2 : 2+int(headerL)])
		return binary.Read(r, binary.LittleEndian, v)
	}
}
func (s *Packet) HeaderToUint64() uint64 {
	if headerL := s.HeaderLen(); headerL == 0 {
		return 0
	} else {
		return binary.LittleEndian.Uint64(s.HeaderRaw())
	}
}
func (s *Packet) UnmarshalHeaderSlice32() (ret []uint32) {
	if headerL := s.HeaderLen(); headerL == 0 {
		return
	} else {
		for i := 2; i < int(headerL); i += 4 {
			v := binary.LittleEndian.Uint32(s.Buff[i : i+4])
			ret = append(ret, v)
		}
	}
	return
}
func (s *Packet) UnmarshalHeaderSlice64() (ret []uint64) {
	if headerL := s.HeaderLen(); headerL == 0 {
		return
	} else {
		for i := 2; i < int(headerL); i += 8 {
			v := binary.LittleEndian.Uint64(s.Buff[i : i+8])
			ret = append(ret, v)
		}
	}
	return
}
func (s *Packet) ReplaceHeader(v interface{}) error {
	w := bytes.NewBuffer(s.Buff[2 : s.HeaderLen()+2])
	w.Reset()
	return binary.Write(w, binary.LittleEndian, v)
}
func (s *Packet) ReplaceHeaderUint64(v uint64) {
	binary.LittleEndian.PutUint64(s.Buff[2:], v)
}

// Unmarshal 将正文解析为变量.会自动识别内部类型并按类型实现解析
func (s *Packet) Unmarshal(v interface{}) error {
	headerL := 2 + uint32(s.HeaderLen())
	typ := s.Type()
	switch typ {
	case TypeJson:
		return json.Unmarshal(s.Buff[headerL:s.Length], v)
	case TypeMsgpack, TypeSpeedTest, TypeTestDelayResponse:
		return msgpack.Unmarshal(s.Buff[headerL:s.Length], v)
	default:
		return fmt.Errorf("can not unmarshal type: %v", typ)
	}
}

// ContentRaw 正文的原始字节
func (s *Packet) ContentRaw() []byte {
	headerL := 2 + uint32(s.HeaderLen())
	return s.Buff[headerL:s.Length]
}
func (s *Packet) WriteContent(v []byte) {
	contentLen := len(v)
	hl := 2 + int(s.HeaderLen())
	totalLen := hl + contentLen
	if cap(s.Buff) < totalLen {
		tmp := make([]byte, hl)
		copy(tmp, s.Buff[:len(tmp)])
		s.Buff = make([]byte, totalLen)
		copy(s.Buff[:len(tmp)], tmp)
	} else {
		s.Buff = s.Buff[0:totalLen]
	}
	copy(s.Buff[hl:], v)
	s.Length = uint32(totalLen)
}

// Bytes 不包含length的全部字节. 类型+头+正文
func (s *Packet) Bytes() []byte {
	return s.Buff[:s.Length]
}

func (s *Packet) Write(typ uint8, header uint64, value interface{}) error {
	var buf []byte
	var err error
	switch typ {
	case TypeJson:
		buf, err = json.Marshal(value)
	case TypeTestDelayRequest, TypeMsgpack, TypeSecretMsgpack:
		buf, err = msgpack.Marshal(value)
	case TypeBinary:
		var ok bool
		buf, ok = value.([]byte)
		if !ok {
			return errors.New("value is not a byte array")
		}
	default:
		return fmt.Errorf("unsupport type: %v", typ)
	}
	if err != nil {
		return err
	}
	length := 10 + len(buf)
	if cap(s.Buff) < length {
		s.Buff = make([]byte, length)
	} else {
		s.Buff = s.Buff[0:length]
	}
	s.Buff[0] = typ
	s.Buff[1] = 8
	binary.LittleEndian.PutUint64(s.Buff[2:10], header)
	copy(s.Buff[10:length], buf)
	s.Length = uint32(length)
	return nil
}
func (s *Packet) Write2(typ uint8, header interface{}, value interface{}) error {
	r := bytes.NewBuffer(s.Buff)
	err := binary.Write(r, binary.LittleEndian, typ) //写入type
	if err != nil {
		return err
	}
	r.WriteByte(0) //写入header长度占位符
	if header != nil {
		err = binary.Write(r, binary.LittleEndian, header) //写入头
		if err != nil {
			return err
		}
		headerL := r.Len() - 2
		if headerL > 0xff {
			return errors.New("header too large")
		}
		s.Buff[1] = uint8(headerL)
	}
	switch typ {
	case TypeBinary:
		if t := reflect.TypeOf(value).Kind(); t != reflect.Array && t != reflect.Slice {
			// 当写的类型为binary时,value不是[]byte就抛错误
			return errors.New("value is not an array")
		}
		err = binary.Write(r, binary.LittleEndian, value)
		if err != nil {
			return err
		}
	case TypeJson:
		buf, err := json.Marshal(value)
		if err != nil {
			return err
		}
		err = binary.Write(r, binary.LittleEndian, buf)
		if err != nil {
			return err
		}
	case TypeMsgpack, TypeSecretMsgpack:
		buf, err := msgpack.Marshal(value)
		if err != nil {
			return err
		}
		err = binary.Write(r, binary.LittleEndian, buf)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupport type: %v", typ)
	}
	s.Length = uint32(r.Len())
	return nil
}
func (s *Packet) Write3(typ uint8, headers []uint64, value interface{}) error {
	if len(headers) == 0 || len(headers) > 30 {
		return errors.New("invalid headers length")
	}
	var buf []byte
	var err error
	switch typ {
	case TypeJson:
		buf, err = json.Marshal(value)
	case TypeTestDelayRequest, TypeMsgpack, TypeSecretMsgpack:
		buf, err = msgpack.Marshal(value)
	case TypeBinary:
		var ok bool
		buf, ok = value.([]byte)
		if !ok {
			return errors.New("value is not a byte array")
		}
	case TypeTerminal:
		var ok bool
		buf, ok = value.([]byte)
		if !ok {
			return errors.New("value is not a byte array")
		}
	default:
		return fmt.Errorf("unsupport type: %v", typ)
	}
	if err != nil {
		return err
	}

	headerLen := len(headers) * 8
	length := 2 + headerLen + len(buf)
	s.Resize(length)
	s.Buff[0] = typ
	s.Buff[1] = uint8(headerLen)

	for i := 0; i < len(headers); i++ {
		binary.LittleEndian.PutUint64(s.Buff[2+i*8:2+(i+1)*8], headers[i])
	}

	copy(s.Buff[2+headerLen:length], buf)
	s.Length = uint32(length)
	return nil
}
func (s *Packet) WriteFn(f func(buf []byte) uint32) {
	if f != nil {
		s.Length = f(s.Buff)
	}
}
func (s *Packet) Resize(size int) {
	if cap(s.Buff) < size {
		s.Buff = append(s.Buff, make([]byte, size-len(s.Buff))...)
	}
	s.Buff = s.Buff[0:size]
}

func (s *Packet) InitWithBytes(v []byte) {
	s.Length = uint32(len(v))
	s.Buff = v
}
