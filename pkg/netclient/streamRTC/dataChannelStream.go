package streamRTC

import (
	"bytes"
	"github.com/pion/webrtc/v4"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// DataChannelStream 只处理了二进制数据,不处理文本数据
type DataChannelStream struct {
	dc    *webrtc.DataChannel
	id    uint16
	label string

	readBuf bytes.Buffer
	readMu  sync.Mutex
	dataCh  chan []byte
	closeCh chan struct{}
	closed  int32

	onOpenCb  func()
	onCloseCb func()

	once sync.Once
}

// NewDataChannelStream 只处理了二进制数据,不处理文本数据
func NewDataChannelStream(dc *webrtc.DataChannel) *DataChannelStream {
	s := &DataChannelStream{
		dc:      dc,
		dataCh:  make(chan []byte, 10),
		closeCh: make(chan struct{}),
	}
	s.dc.OnOpen(s._onOpen)
	s.dc.OnMessage(s.onMessage)
	s.dc.OnClose(s._onClose)
	return s
}
func (s *DataChannelStream) onMessage(msg webrtc.DataChannelMessage) {
	if atomic.LoadInt32(&s.closed) != 0 {
		return
	}
	select {
	case s.dataCh <- msg.Data:
	}
}

func (s *DataChannelStream) onOpen(f func()) {
	s.onOpenCb = f
}
func (s *DataChannelStream) _onOpen() {
	if i := s.dc.ID(); i != nil {
		s.id = *i
	}
	s.label = s.dc.Label()

	if s.onOpenCb != nil {
		s.onOpenCb()
	}
}
func (s *DataChannelStream) _onClose() {
	if s.onCloseCb != nil {
		go s.onCloseCb()
	}
	s.Close()
}
func (s *DataChannelStream) onClose(f func()) {
	s.onCloseCb = f
}
func (s *DataChannelStream) ID() uint16 {
	return s.id
}
func (s *DataChannelStream) Label() string {
	return s.label
}

// GetClosed 检测是否关闭 true=已关闭
func (s *DataChannelStream) GetClosed() bool {
	return atomic.LoadInt32(&s.closed) == 0
}
func (s *DataChannelStream) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IP{127, 0, 0, 1}, Port: 0}
}

// Read 只处理了二进制数据,不处理文本数据
func (s *DataChannelStream) Read(p []byte) (n int, err error) {
	if atomic.LoadInt32(&s.closed) != 0 {
		return 0, io.ErrClosedPipe
	}
	s.readMu.Lock()
	defer s.readMu.Unlock()
	if s.readBuf.Len() > 0 {
		return s.readBuf.Read(p)
	}
	select {
	case data := <-s.dataCh:
		s.readBuf.Write(data)
		return s.readBuf.Read(p)
	case <-s.closeCh:
		return 0, io.EOF
	}
}
func (s *DataChannelStream) Write(p []byte) (n int, err error) {
	if atomic.LoadInt32(&s.closed) != 0 {
		return 0, io.ErrClosedPipe
	}
	for len(p) > 0 {
		chunkSize := len(p)
		if chunkSize > _maxPackSize {
			chunkSize = _maxPackSize
		}
		err = s.dc.Send(p[:chunkSize])
		if err != nil {
			return
		}
		n += chunkSize
		p = p[chunkSize:]
	}
	return
}
func (s *DataChannelStream) SetReadDeadline(t time.Time) error {
	return nil
}
func (s *DataChannelStream) SetWriteDeadline(t time.Time) error {
	return nil
}
func (s *DataChannelStream) Close() error {
	atomic.StoreInt32(&s.closed, 1)
	s.once.Do(func() {
		close(s.closeCh)
		close(s.dataCh)
	})
	return nil
}
