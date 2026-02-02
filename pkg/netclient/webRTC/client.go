package webRTC

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	Attach        interface{} //附加数据，可以用来做标志，比如这条连接的用户Id，等
	AttachString  string      //附加的字符串文本，比如token字符串
	signalOnline  int32
	addr          string
	signal        *websocket.Conn
	signalOnClose func(c *websocket.Conn)
	signalTimeout time.Duration
	signalLock    sync.Mutex

	isGuest    bool
	peerConn   *webrtc.PeerConnection
	connConfig *webrtc.Configuration
	remoteAddr string

	once             sync.Once
	channel          sync.Map //map[uint6]{string,*webrtc.DataChannel}
	onChannelMessage OnDataChannelCallback
	onChannelOpen    StandardCallback
	onChannelClose   StandardCallback
	onClose          func(c *Client)
	logger           LoggerFunc
}

func NewClient(urlString string, token string, isGuest bool, onChannelMessage OnDataChannelCallback) *Client {
	return &Client{
		signalTimeout:    time.Second * 10,
		onChannelMessage: onChannelMessage,
		isGuest:          isGuest,
		addr:             fmt.Sprintf("%s?token=%s", urlString, url.QueryEscape(token)),
	}
}
func (s *Client) Start() error {
	if atomic.LoadInt32(&s.signalOnline) == 1 {
		return nil
	}
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	conn, _, err := dialer.Dial(s.addr, nil)
	if err != nil {
		return err
	}
	s.signal = conn
	go s.onSignalData(conn)
	return nil
}
func (s *Client) onSignalData(c *websocket.Conn) {
	atomic.StoreInt32(&s.signalOnline, 1)
	defer func() {
		atomic.StoreInt32(&s.signalOnline, 0)
		if s.signalOnClose != nil {
			s.signalOnClose(c)
		}
		if s.onClose != nil {
			if s.peerConn != nil && s.peerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
				return
			}
			s.Close()
		}
	}()
	_ = c.SetReadDeadline(time.Now().Add(s.signalTimeout))
	i := int32(0)
	for {
		typ, payload, err := c.ReadMessage()
		if err != nil {
			s.log(err.Error())
			_ = c.Close()
			return
		}
		_ = c.SetReadDeadline(time.Now().Add(s.signalTimeout))
		if typ == websocket.BinaryMessage {
			if err := s.sendSignalBin(payload); err != nil {
				s.log(err.Error())
				return
			}
			continue
		}
		var msg Message
		if err = json.Unmarshal(payload, &msg); err != nil {
			s.log("parser payload to message err,%s", err.Error())
			continue
		}
		s.log("%v", msg)
		switch msg.Fun {
		case FunHandshake: //ws鉴权成功后，服务端会立即发Handshake
			if s.connConfig, err = s.createPeerConnConfig(&msg); err != nil {
				s.log(err.Error())
			}
			if s.isGuest {
				//如果身份是主动发起者，向服务器询问被访问方是否在线
				msg.Fun = FunHostOnline
				msg.Code = 200
				if err = s.sendSignal(&msg); err != nil {
					s.log(err.Error())
					return
				}
			}
		case FunHostOnline: //服务端返回对端是否在线
			if msg.Code == 200 { //对端在线或上线
				if err = s.createPeerConn(); err != nil {
					msg.Code = 400
					if err = s.sendSignal(&msg); err != nil {
						s.log(err.Error())
						return
					}
				} else {
					if d, err := s.CreateDataChannel(""); err == nil {
						s.onCreateChannel(d)
					}
					msg = *s.createOffer(&msg)
					if err = s.sendSignal(&msg); err != nil {
						s.log(err.Error())
						return
					}
				}
			} else {
				//对端不在线
				if atomic.AddInt32(&i, 1) > 100 {
					c.Close()
				}
				time.AfterFunc(time.Millisecond*100, func() {
					msg.Fun = FunHostOnline
					msg.Code = 200
					if err = s.sendSignal(&msg); err != nil {
						s.log(err.Error())
						return
					}
				})
			}
		case FunOffer: //收到offer说明是host身份,创建answer
			if err = s.createPeerConn(); err != nil {
				msg.Code = 400
				if err = s.sendSignal(&msg); err != nil {
					s.log(err.Error())
					return
				}
			} else {
				msg = *s.createAnswer(&msg)
				if err = s.sendSignal(&msg); err != nil {
					s.log(err.Error())
					return
				}
			}
			break
		case FunAnswer: //收到answer
			_ = s.setAnswer(&msg)
		case FunCandidate:
			if err = s.addICECandidate(&msg); err != nil {
				s.log(err.Error())
			}
			break
		default:
			s.log("unknown message field Fun=%s", msg.Fun)
			msg.Code = 400
			msg.Msg = fmt.Sprintf("unknown message field Fun=%s", msg.Fun)
			if err = s.sendSignal(&msg); err != nil {
				s.log(err.Error())
				return
			}
		}
	}
}
func (s *Client) sendSignal(msg *Message) error {
	s.signalLock.Lock()
	defer s.signalLock.Unlock()
	if s.signal == nil || atomic.LoadInt32(&s.signalOnline) == 0 {
		return errors.New("offline")
	}
	return s.signal.WriteMessage(websocket.TextMessage, msg.Marshal())
}
func (s *Client) sendSignalBin(buf []byte) error {
	s.signalLock.Lock()
	defer s.signalLock.Unlock()
	if s.signal == nil || atomic.LoadInt32(&s.signalOnline) == 0 {
		return errors.New("offline")
	}
	return s.signal.WriteMessage(websocket.BinaryMessage, buf)
}
func (s *Client) createPeerConnConfig(msg *Message) (*webrtc.Configuration, error) {
	var v ICEServerInfo
	if err := msg.Unmarshal(&v); err != nil {
		return nil, err
	}
	s.log("ice server info: %v", v)
	return &webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs:           v.Urls,
				Username:       v.Username,
				Credential:     v.Password,
				CredentialType: webrtc.ICECredentialTypePassword,
			},
		},
	}, nil
}
func (s *Client) createPeerConn() error {
	if s.connConfig == nil {
		return errors.New("peer connection Configuration is nil")
	}
	peerConn, err := webrtc.NewPeerConnection(*s.connConfig)
	if err != nil {
		return err
	}
	if s.peerConn != nil {
		_ = s.peerConn.Close()
	}
	s.peerConn = peerConn
	peerConn.OnICECandidate(s.onICECandidate)
	peerConn.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		s.log("ICE Connection State has changed: %s", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateDisconnected || connectionState == webrtc.ICEConnectionStateFailed {
			// ice状态为断开时关闭整个实例
			s.Close()
		} else if connectionState == webrtc.ICEConnectionStateConnected {
			iceTransport := peerConn.SCTP().Transport().ICETransport()
			pair, e := iceTransport.GetSelectedCandidatePair()
			if e == nil {
				s.remoteAddr = fmt.Sprintf("%s:%d", pair.Remote.Address, pair.Remote.Port)
			}
		}
	})
	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateClosed ||
			state == webrtc.PeerConnectionStateFailed {
			s.Close()
		}
		s.log("peer connection state changed to %s", state.String())
	})
	peerConn.OnDataChannel(s.onCreateChannel)
	return nil
}
func (s *Client) onCreateChannel(d *webrtc.DataChannel) {
	var channelId uint16
	var label string
	ch := DataChannel{
		channel:  d,
		length:   0,
		received: 0,
		data:     nil,
		lock:     sync.Mutex{},
		doLock:   sync.Mutex{},
	}
	d.OnOpen(func() {
		channelId = *d.ID()
		label = d.Label()
		s.channel.Store(channelId, &ch)
		s.log("Data channel open id=%d label='%s'", channelId, label)
		if s.onChannelOpen != nil {
			s.onChannelOpen(s, &ch)
		}
	})
	d.OnMessage(func(msg webrtc.DataChannelMessage) {
		if channelId == 0 {
			channelId = *d.ID()
			label = d.Label()
		}
		if msg.IsString {
			if s.onChannelMessage != nil {
				s.onChannelMessage(s, channelId, label, &msg)
				return
			}
			return
		}
		if err := ch.Do(s, msg.Data, s.onChannelMessage); err != nil {
			s.log(err.Error())
		}
	})
	d.OnClose(func() {
		s.channel.Delete(channelId)
		s.log("DataChannel closed id=%d label='%s'", channelId, label)
		if s.onChannelClose != nil {
			s.onChannelClose(s, &ch)
		}
	})
}
func (s *Client) onDataChannelMsg(channelId uint16, label string, msg *webrtc.DataChannelMessage) error {
	if msg.IsString {
		if s.onChannelMessage != nil {
			s.onChannelMessage(s, channelId, label, msg)
			return nil
		}
		return errors.New("on message callback is nil")
	}
	channel, err := s.GetDataChannel(channelId, "")
	if err != nil {
		return err
	}
	return channel.Do(s, msg.Data, s.onChannelMessage)
}
func (s *Client) setAnswer(msg *Message) error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	var v webrtc.SessionDescription
	if err := msg.Unmarshal(&v); err != nil {
		return err
	}
	return s.peerConn.SetRemoteDescription(v)
}
func (s *Client) createOffer(msg *Message) *Message {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	msg.Fun = FunOffer
	offer, err := s.peerConn.CreateOffer(nil)
	if err != nil {
		msg.Code = 400
		msg.Msg = err.Error()
		msg.Data = nil
		return msg
	}
	if err = s.peerConn.SetLocalDescription(offer); err != nil {
		msg.Code = 400
		msg.Msg = err.Error()
		msg.Data = nil
		return msg
	}
	msg.Code = 200
	msg.Msg = "success"
	_ = msg.SetData(&offer)
	return msg
}
func (s *Client) createAnswer(msg *Message) *Message {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	msg.Fun = FunAnswer
	var v webrtc.SessionDescription
	if err := msg.Unmarshal(&v); err != nil {
		msg.Code = 400
		msg.Msg = err.Error()
		return msg
	}
	if err := s.peerConn.SetRemoteDescription(v); err != nil {
		msg.Code = 400
		msg.Msg = err.Error()
		return msg
	}
	answer, err := s.peerConn.CreateAnswer(nil)
	if err != nil {
		msg.Code = 400
		msg.Msg = err.Error()
		return msg
	}
	if err = s.peerConn.SetLocalDescription(answer); err != nil {
		msg.Code = 400
		msg.Msg = err.Error()
		return msg
	}
	msg.Code = 200
	msg.Msg = "success"
	_ = msg.SetData(&answer)
	return msg
}
func (s *Client) addICECandidate(msg *Message) error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	var v webrtc.ICECandidateInit
	if err := msg.Unmarshal(&v); err != nil {
		return err
	}
	s.log("add ice candidate %v", v)
	return s.peerConn.AddICECandidate(v)
}
func (s *Client) onICECandidate(candidate *webrtc.ICECandidate) {
	if candidate != nil {
		res := &Message{Fun: FunCandidate, Code: 200}
		ci := candidate.ToJSON()
		s.log("on ice candidate %v", ci)
		_ = res.SetData(&ci)
		if err := s.sendSignal(res); err != nil {
			s.log(err.Error())
		}
	}
}

// GetPeerConnectionStat 取连接实例的状态
func (s *Client) GetPeerConnectionStat() webrtc.PeerConnectionState {
	if s.peerConn != nil {
		return s.peerConn.ConnectionState()
	}
	return webrtc.PeerConnectionStateFailed
}

// GetPeerConnection 返回webrtc的PeerConnection连接实例
func (s *Client) GetPeerConnection() *webrtc.PeerConnection {
	return s.peerConn
}

// CreateDataChannel Guest方创建数据通道
func (s *Client) CreateDataChannel(label string) (*webrtc.DataChannel, error) {
	if !s.isGuest {
		return nil, errors.New("only guest can create DataChannel")
	}
	if s.peerConn != nil {
		d, err := s.peerConn.CreateDataChannel(label, nil)
		if err != nil {
			return nil, err
		}
		// s.channel.Store(*d.ID(),d) //todo 待验证
		return d, err
	}
	return nil, errors.New("peer connection nil")
}

// GetDataChannel 获取数据通道对象，channelId大于0按channelId查询，忽略label，否则按label查询。
// 推荐使用此方案，获取实例到外部去发送数据。
func (s *Client) GetDataChannel(channelId uint16, label string) (*DataChannel, error) {
	var ok, find bool
	var channel *DataChannel
	if channelId == 0 {
		s.channel.Range(func(key, value any) bool {
			if channel, ok = value.(*DataChannel); ok {
				if channel.Label() == label {
					find = true
					return false
				}
			}
			return true
		})
	} else {
		if value, ok := s.channel.Load(channelId); ok {
			if channel, ok = value.(*DataChannel); ok {
				find = true
			}
		}
	}
	if find {
		return channel, nil
	}
	return nil, errors.New("not found")
}

// Send 发送[]byte到指定的数据通道，效率略低，不推荐用来发送大量数据。
// 发送大量数据使用GetDataChannel获取通道到外部去批量发送
func (s *Client) Send(channelId uint16, data []byte) error {
	channel, err := s.GetDataChannel(channelId, "")
	if err != nil {
		return err
	}
	return channel.Send(data)
}

// SendText 发送string到指定的数据通道，效率略低，不推荐用来发送大量数据。
// 发送大量数据使用GetDataChannel获取通道到外部去批量发送
func (s *Client) SendText(channelId uint16, data string) error {
	channel, err := s.GetDataChannel(channelId, "")
	if err != nil {
		return err
	}
	return channel.SendText(data)
}

// TunnelMode 取隧道当前的工作模式：direct,p2p,relay,unknown
func (s *Client) TunnelMode() ConnectMode {
	if s.peerConn != nil {
		st := s.peerConn.GetStats()
		v := StatsReport{st}
		return v.Mode()
	}
	return ConnectMode(0)
}

// CloseChannel 关闭指定的数据通道
func (s *Client) CloseChannel(channelId uint16, label string) {
	channel, err := s.GetDataChannel(channelId, label)
	if err != nil {
		return
	}
	channel.Close()
	s.channel.Delete(channelId)
}
func (s *Client) RemoteAddr() string {
	return s.remoteAddr
}

// SetOnChannelOpen 置数据通道打开回调函数
func (s *Client) SetOnChannelOpen(f StandardCallback) {
	s.onChannelOpen = f
}
func (s *Client) SetOnClose(f func(c *Client)) {
	s.onClose = f
}

// SetOnChannelClose 置数据通道关闭回调函数
func (s *Client) SetOnChannelClose(f StandardCallback) {
	s.onChannelClose = f
}
func (s *Client) SetOnSignalConnectionClose(f func(c *websocket.Conn)) {
	s.signalOnClose = f
}
func (s *Client) GetSignalState() bool {
	return atomic.LoadInt32(&s.signalOnline) == 1
}
func (s *Client) Close() {
	if s.signal != nil {
		_ = s.signal.Close()
	}
	s.channel.Range(func(key, value any) bool {
		if ch, ok := value.(*webrtc.DataChannel); ok {
			_ = ch.Close()
		}
		s.channel.Delete(key)
		return true
	})
	if s.peerConn != nil {
		_ = s.peerConn.Close()
	}
	s.once.Do(func() {
		if s.onClose != nil {
			go s.onClose(s)
		}
	})
	s.onChannelClose = nil
	s.onChannelOpen = nil
	s.onChannelMessage = nil
}
func (s *Client) log(f interface{}, v ...interface{}) {
	if s.logger != nil {
		s.logger(f, v...)
	}
}
func (s *Client) SetLogger(l LoggerFunc) {
	s.logger = l
}
