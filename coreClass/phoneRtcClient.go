package coreClass

import (
	"encoding/json"
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/BufferRTC"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/netclient"
	"github.com/ghp3000/public"

	"log"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

func (s *DeviceInfo) RtcH264Connect(tokenInfo public.TokenInfo) {
	logs.Info("控制窗口rtc开始连接,deviceId=%d,GuestUrl=%s,Token=%s", s.DeviceId, tokenInfo.GuestUrl, tokenInfo.Token)
	s.Time连接成功时间戳 = -1
	_, err := BufferRTC.New(s.DeviceId, tokenInfo.GuestUrl, tokenInfo.Token, true, bufferPool.Buffer, s.onRtcH264Open, s.onRtcH264Close, s.onRtcH264Data)
	if err != nil {
		logs.Error("rtc err:%s,TokenInfo=%v", err.Error(), tokenInfo)
		if atomic.LoadInt32(&s.WindowIsOpen) == 1 {
			logs.Info("控制窗口rtc连接断开,重连")
		}
		return
	}

}

func (s *DeviceInfo) onRtcH264Open(c netclient.NetClient) {
	go s.Rtc线程处理客户端连接(c)
}
func (s *DeviceInfo) onRtcH264Close(c netclient.NetClient) {
	go s.Rtc线程调用客户端断开(c)
}
func (s *DeviceInfo) Rtc线程调用客户端断开(c netclient.NetClient) {
	logs.Info("控制窗口rtc连接断开", c.Extra().(uint64))

}
func (s *DeviceInfo) Rtc线程处理客户端连接(c netclient.NetClient) {
	logs.Info("控制窗口RtcH264连接成功", c.Extra().(uint64))
	s.RtcConn = c
	atomic.StoreInt32(&s.Heartbeat, 1)
	client, okk := c.(*BufferRTC.RtcClient)
	if okk {
		//		logs.Info("控制窗口当前打洞的工作模式=========================================================================为:%s", client.Client.TunnelMode())
		phone, ok := S.DidGetPhoneInfo(s.DeviceId)
		if ok {
			phone.RtcType = fmt.Sprintf("连接模式[%s]", client.Client.TunnelMode())
			atomic.StoreInt64(&phone.Time连接成功时间戳, time.Now().UnixMilli())
			logs.Info("Rtc线程处理客户端连接Rtc线程处理客户端连接Rtc线程处理客户端连接", atomic.LoadInt64(&phone.Time连接成功时间戳))
			S.DidSaveDeviceInfo(s.DeviceId, phone)
		}

	}

	if atomic.LoadInt32(&s.WindowIsOpen) == 0 {
		logs.Info("[控制窗口Rtc连接成功回调]控制窗口未打开,不启动H264解码，关闭连接")
		err := s.RtcConn.Close()
		if err != nil {
			logs.Error("关闭Rtc连接失败:%s", err.Error())
			return
		}
	}
	//	logs.Info("Rtc线程处理客户端连接,处理结束%d", c.Extra().(uint64))
}

func (s *DeviceInfo) onRtcH264Data(packet *bufferPool.Packet, conn netclient.NetClient) bool {
	//logs.Info("控制窗口rtc收到数据,type=%d消息进入阿斯达手动阀+++++++++++++++++++++++++++++++++++++++++++", packet.Type())
	typ := packet.Type()
	switch typ {
	case bufferPool.TypePing:
		if err := conn.SendPong(); err != nil {
			err = conn.Close()
			if err != nil {
				logs.Error("控制窗口Rtc发送Pong失败:%s", err.Error())
			}
		}
	case bufferPool.TypePong:
		atomic.StoreInt32(&s.Heartbeat, 1)
		break
	case bufferPool.TypeMsgpack:
		deviceId := conn.Extra().(uint64)
		var msg public.Message
		if err := packet.Unmarshal(&msg); err != nil {
			logs.Error("控制窗口rtc收到的msgpack数据反序列化失败,", err)
		}
		msg.Type = typ
		go s.收到MsgPack或Json信息(&msg, deviceId)
	case bufferPool.TypeTestDelayResponse:
		var v DelayRequest
		if err := packet.Unmarshal(&v); err != nil {
			logs.Error("控制窗口rtc返回的消息反序列化失败,", err)
			return true
		}
		logs.Info("控制窗口rtc异步测速的消息回来了,延迟=%d微秒", time.Now().UnixMicro()-v.Timestamp)
	case bufferPool.TypeVideo: //收到H264数据
		//log.Println("收到H264数据,deviceId=", conn.Extra().(uint64))
		s.收到H264数据处理(packet.ContentRaw())
	case bufferPool.TypeAudio:
		//s.AudioPlayer.Chan <- packet.Ref()
		s.Rtc收到音频数据(packet.Ref())
	default:
		logs.Error("控制窗口rtcRtc未处理的类型: %d", typ)
	}
	return true
}
func (s *DeviceInfo) 收到MsgPack或Json信息(msg *public.Message, deviceId uint64) {
	//	logs.Info("控制窗口rtc收到msgpack的消息:  F=%s, deviceId=%d, code=%d,msg=%s", msg.F, deviceId, msg.Code, msg.Msg)
	switch msg.F {
	case public.FuncTestDelay:
		var v DelayRequest
		if err := msg.Unmarshal(&v); err != nil {
			logs.Error("控制窗口rtc返回的消息反序列化失败,", err)
		}

		//		logs.Info("控制窗口rtc异步测速的消息回来了,延迟=%sms", "["+strconv.FormatInt((time.Now().UnixMicro()-v.Timestamp)/1000, 10)+"]ms")
		atomic.StoreInt64(&s.TimeDelay, (time.Now().UnixMicro()-v.Timestamp)/1000)
		return
	case public.FuncScreenChange:
		s.收到屏幕旋转事件(deviceId, msg)
		return
	case public.FuncStartVideo:
		//		logs.Info("控制窗口rtc收到H264命令执行结果的消息: F=%d, deviceId=%d, code=%d, 执行结果=%s", msg.Type, deviceId, msg.Code, msg.Msg)
		if msg.Code == 0 {
			atomic.StoreInt32(&s.H264IsOpen, 1)
			atomic.StoreInt32(&s.H264开启判断计次, 0)
		} else {
			atomic.StoreInt32(&s.H264IsOpen, 0)
			atomic.StoreInt32(&s.H264开启判断计次, 0)
		}
		go s.Rtc切换输入法(S.输入法包名)
		return
	case public.FuncStopAudio:
		logs.Info("控制窗口rtc收到StopAudio命令执行结果的消息: F=%d, deviceId=%d, code=%d, 执行结果=%s", msg.Type, deviceId, msg.Code, msg.Msg)
		return
	case public.FuncStartAudio:
		logs.Info("控制窗口rtc收到StartAudio命令执行结果的消息: F=%d, deviceId=%d, code=%d, 执行结果=%s", msg.Type, deviceId, msg.Code, msg.Msg)
		atomic.StoreInt32(&s.H264IsOpen, 0)
		atomic.StoreInt32(&s.H264开启判断计次, 0)
		return
	case public.FuncImg:
		logs.Info("控制窗口rtc收到截图命令执行结果的消息: F=%d, deviceId=%d, code=%d, 执行结果=%s", msg.Type, deviceId, msg.Code, msg.Msg)
		s.Rtc收到截图图片(msg, deviceId)
		return
	case public.FuncGetText:
		s.Rtc收到手机剪辑版内容(msg)
		return
	default:
		var a interface{}
		err := json.Unmarshal(msg.DataMsgpack, &a)
		if err != nil {

		}
		logs.Error("控制窗口rtc未处理的函数 F=%s, deviceId=%d, code=%d,msg=%s,%v", msg.F, deviceId, msg.Code, msg.Msg, a)
		return
	}
}
func (s *DeviceInfo) Rtc收到手机剪辑版内容(msg *public.Message) {
	var rec string
	err := msg.Unmarshal(&rec)
	if err != nil {
		logs.Error("[Rtc收到手机剪辑版内容]解析手机剪辑版内容失败: %v", err)
	}
	logs.Info("[Rtc收到剪辑版内容]", rec)
	err = clipboard.WriteAll(rec)
	if err != nil {
		logs.Error("[Rtc收到手机剪辑版内容]置剪辑版内容失败: %v", err)
	}
}
func (s *DeviceInfo) 收到屏幕旋转事件(deviceId uint64, msg *public.Message) {
	//logs.Info("控制窗口rtc收到屏幕旋转事件,设备编号:%d", deviceId, msg.Type, string(msg.DataMsgpack))
	type Orientation struct {
		Orientation int `json:"orientation" msgpack:"orientation"`
	}
	var rec Orientation
	err := msg.Unmarshal(&rec)
	if err != nil {
		logs.Error("[MiddleRtcClient]解析屏幕旋转事件失败: %v", err)
		return
	}
	s.Orientation = rec.Orientation
	//s.OrientationCallback(rec.Orientation)
	logs.Info("解析屏幕旋转事件%d,deviceId%d", rec.Orientation, deviceId)

}
func (s *DeviceInfo) Rtc收到音频数据(p *bufferPool.Packet) {
	//	logs.Info("收到音频数据,deviceId=%d", s.DeviceId, len(p.ContentRaw()))

}
func (s *DeviceInfo) 收到H264数据处理(data []byte) {
	//logs.Info("收到H264数据,deviceId=%d,len=%d根=======================================", s.DeviceId, len(data))
	if atomic.LoadInt32(&s.WindowISOpened) == 0 {
		logs.Info("收到H264数据,deviceId=%d,len=%d,窗口未打开", s.DeviceId, len(data))
		if s.RtcConn != nil {
			err := s.RtcConn.Close()
			if err != nil {
				logs.Error("关闭Rtc连接失败:%s", err.Error())
			}
		}
		return
	}

	if atomic.LoadInt32(&s.WindowIsOpen) == 1 {
		//		logs.Info("收到H264数据,deviceId=%d,len=%d", s.DeviceId, len(data))

		//		logs.Info("收到H264数据,deviceId=%d,len=%d", s.DeviceId, len(data))
		//t := time.Now().UnixMicro()

		//		logs.Error("[MiddleRtcClient]<收到H264数据处理,成功>: %v", err)
	}
}
func (s *DeviceInfo) Rtc发送鼠标滚轮消息(conn netclient.NetClient, deviceIds []uint64, upOrDown int, x int, y int) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncScroll, atomic.AddUint32(&S.MsgSeq, 1))
	mouseReq := Mousescroll{
		UpOrDown: upOrDown,
		X:        x,
		Y:        y,
	}

	if err := msg.Marshal(&mouseReq); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
	if err != nil {
		return
	}
	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("发送触屏操作失败,%s", err.Error())
			return
		}
	}
}
func (s *DeviceInfo) Rtc发送键盘消息(conn netclient.NetClient, deviceIds []uint64, keyCode int, action int) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncKey, atomic.AddUint32(&S.MsgSeq, 1))
	keyboardReq := keyboard{
		KeyCode: keyCode,
		Action:  action,
	}
	var err error
	if err = msg.Marshal(&keyboardReq); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	err = pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
	if err != nil {
		return
	}
	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("发送触屏操作失败,%s", err.Error())
			return
		}
	}
}
func (s *DeviceInfo) Rtc触屏操作(conn netclient.NetClient, deviceIds []uint64, touchType int, x int, y int, offset int, pressure int, id int) {
	//{f:4,data:[ {type:0, x: 400, y: 500,offset:10,pressure:1,id:1}],req:true,seq:1}
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncTouch, atomic.AddUint32(&S.MsgSeq, 1))
	imgReq := TouchRequest{
		Type:     touchType,
		X:        x,
		Y:        y,
		Offset:   offset,
		Pressure: pressure,
		Id:       id,
	}

	//if s.Orientation == 2 {
	//	X := imgReq.X
	//	Y := imgReq.Y
	//	imgReq.X = Y
	//	imgReq.Y = 1000 - X
	//	//log.Println("触屏操作", imgReq.X, imgReq.Y)
	//}

	var imgReqs []TouchRequest
	imgReqs = append(imgReqs, imgReq)

	if err := msg.Marshal(&imgReqs); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
	if err != nil {
		return
	}
	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("发送触屏操作失败,%s", err.Error())
			return
		}
	}

}

func (s *DeviceInfo) Rtc开启H264串流() {
	if s.Online&2 > 0 {

	} else {
		logs.Error("<手机离线>")
		return
	}

	width := s.DeviceWidth
	height := s.DeviceHeight
	if width <= 0 {
		width = 1280 // 默认宽度
	}
	if height <= 0 {
		height = 720 // 默认高度
	}
	//	logs.Info("控制端deviceId=%d，Rtc开启H264投屏", s.DeviceId)
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncStartVideo, atomic.AddUint32(&S.MsgSeq, 1))

	streamReq := H264StreamRequest{
		FPS:     30,
		Bitrate: 4000000,
		Quality: 10,
		Width:   width / 2,
	}
	if err := msg.Marshal(&streamReq); err != nil {
		logs.Error("控制窗口[%d]序列化H264投屏请求消息失败,%s", s.DeviceId, err.Error())
		return
	}
	if s.RtcConn == nil {
		logs.Error("控制窗口[%d]Rtc连接未建立,无法发送H264投屏请求消息", s.DeviceId)
		return
	}
	if err := s.RtcConn.SendMsgpack(msg, 0); err != nil {
		logs.Error("控制窗口[%d]发送H264投屏请求消息失败,%s", s.DeviceId, err.Error())
		return
	} else {
		logs.Info("控制窗口[%d]已发送H264投屏请求: 帧率=%d, 码率=%d, 质量=%d, 宽度=%d", s.DeviceId, streamReq.FPS, streamReq.Bitrate, streamReq.Quality, streamReq.Width)
	}

}
func (s *DeviceInfo) Rtc延迟检测() {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncTestDelay, atomic.AddUint32(&S.MsgSeq, 1))
	test := DelayRequest{time.Now().UnixMicro()}
	if err := msg.Marshal(&test); err != nil {
		logs.Error("序列化message的data字段出错了,%s", err.Error())
	}
	if s.RtcConn == nil {
		logs.Error("Rtc连接未建立,无法发送延迟检测消息")
		return
	}
	if err := s.RtcConn.SendMsgpack(msg, 0); err != nil {
		logs.Error("发送消息出错了,%s", err.Error())
	}
}
func (s *DeviceInfo) Rtc输入法输入文本(conn netclient.NetClient, deviceIds []uint64, text string) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncInputText, atomic.AddUint32(&S.MsgSeq, 1))
	methodInputReq := methodInput{
		Text: text,
	}

	if err := msg.Marshal(&methodInputReq); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
	if err != nil {
		return
	}
	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("发送触屏操作失败,%s", err.Error())
			return
		}
	}
}
func (s *DeviceInfo) Rtc取远程设备剪辑版内容(conn netclient.NetClient, deviceIds []uint64) {
	var msg *public.Message
	msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncGetText, atomic.AddUint32(&S.MsgSeq, 1))
	msg.Req = true

	pkt := bufferPool.Get()
	defer pkt.Put()
	err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
	if err != nil {
		return
	}
	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("发送触屏操作失败,%s", err.Error())
			return
		}
	}

}
func (s *DeviceInfo) Rtc发送Shell命令(conn netclient.NetClient, deviceIds []uint64, shell string, isreturn bool) {
	var msg *public.Message
	if isreturn == true {
		msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncCMDWithResult, atomic.AddUint32(&S.MsgSeq, 1))
	} else {
		msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncCMD, atomic.AddUint32(&S.MsgSeq, 1))
	}
	shellcmdReq := shellcmd{
		Shell: shell,
	}
	if err := msg.Marshal(&shellcmdReq); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
	if err != nil {
		return
	}
	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("发送触屏操作失败,%s", err.Error())
			return
		}
	}

}
func (s *DeviceInfo) Rtc开启声音() {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncStartAudio, atomic.AddUint32(&S.MsgSeq, 1))
	streamReq := AudioStreamRequest{
		SampleRate:   48000,
		AudioBitRate: 128000,
	}
	if err := msg.Marshal(&streamReq); err != nil {
		logs.Error("控制窗口[%d]序列化声音请求消息失败,%s", s.DeviceId, err.Error())
		return
	}
	if s.RtcConn == nil {
		logs.Error("控制窗口[%d]Rtc连接未建立,无法发送声音请求消息", s.DeviceId)
		return
	}
	if err := s.RtcConn.SendMsgpack(msg, 0); err != nil {
		logs.Error("控制窗口[%d]发送声音请求消息失败,%s", s.DeviceId, err.Error())
		return
	} else {
		logs.Info("控制窗口[%d]已发送声音请求: 采样率=%d, 码率=%d", s.DeviceId, streamReq.SampleRate, streamReq.AudioBitRate)
	}

}
func (s *DeviceInfo) Rtc关闭声音() {
	//s.CloseAudioPlayer()
	atomic.AddUint32(&S.MsgSeq, 1)
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncStopAudio, atomic.LoadUint32(&S.MsgSeq))
	msg.Seq = atomic.LoadUint32(&S.MsgSeq)
	if s.RtcConn == nil {
		logs.Error("控制窗口[%d]Rtc连接未建立,无法发送关闭声音请求", s.DeviceId)
		return
	}
	if err := s.RtcConn.SendMsgpack(msg, 0); err != nil {
		logs.Error("控制窗口[%d]发送声音关闭声音失败,%s", s.DeviceId, err.Error())
		return
	} else {
		logs.Info("控制窗口[%d]已发送关闭声音请求", s.DeviceId)
	}
}
func (s *DeviceInfo) Rtc关闭H264() {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncStopVideo, atomic.AddUint32(&S.MsgSeq, 1))

	if s.RtcConn == nil {
		logs.Error("控制窗口[%d]Rtc连接未建立,无法发送H264关闭请求消息", s.DeviceId)
		return
	}
	if err := s.RtcConn.SendMsgpack(msg, 0); err != nil {
		logs.Error("控制窗口[%d]发送H264投屏请求消息失败,%s", s.DeviceId, err.Error())
		return
	} else {
		logs.Info("控制窗口[%d]已发送H264投屏请求: 帧率=%d, 码率=%d, 质量=%d, 宽度=%d", s.DeviceId)
	}

}
func (s *DeviceInfo) Rtc根据链接取图(conn netclient.NetClient, diviceIds []uint64, Width int) {
	//	logs.Info("批量取图,diviceIds=%v,Width=%d", diviceIds, Width)
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncImg, atomic.AddUint32(&S.MsgSeq, 1))

	type ImageRequest struct {
		Width  int `json:"width" msgpack:"width"`
		Height int `json:"height" msgpack:"height"`
		Qua    int `json:"qua" msgpack:"qua"`
		Scale  int `json:"scale" msgpack:"scale"`
		X      int `json:"x" msgpack:"x"`
		Y      int `json:"y" msgpack:"y"`
		Type   int `json:"type" msgpack:"type"`
	}
	imgReq := ImageRequest{
		Width:  0,
		Height: 0,
		Qua:    70,
		Scale:  Width,
		X:      0,
		Y:      0,
		Type:   S.Rtc屏幕墙取图方式,
	}
	if err := msg.Marshal(&imgReq); err != nil {
		logs.Error("Rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//		logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//	logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	err := pkt.Write3(bufferPool.TypeMsgpack, diviceIds, &msg)
	if err != nil {
		logs.Error("Rtc序列化图片请求消息失败,%s,", err.Error(), diviceIds)
		return
	} else {
		//		logs.Info("序列化图片请求消息成功,msg=%v", msg)
	}
	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("Rtc发送图片请求消息失败,%s", err.Error())
			return
		} else {
			//		logs.Info("发送图片请求消息成功,msg=%v", msg)
		}
	}

	return
}
func (s *DeviceInfo) Rtc收到截图图片(msg *public.Message, deviceId uint64) {

	if msg.Code != 0 {
		logs.Error("公共rtc获取图片失败,msg=%s", msg.DataMsgpack)
		return
	}
	var imgData []byte
	if err := msg.Unmarshal(&imgData); err != nil {
		logs.Error("公共rtc收到的图片数据反序列化失败,", err)
		return
	}
	if len(imgData) > 16 {
		_, ok := S.DidGetPhoneInfo(deviceId)
		if ok {
			if S.Rtc屏幕墙取图方式 == 2 {
				jpg, err := S.webp转Png(imgData, 100)

				if err == nil {
					if _, err := os.Stat("screen"); os.IsNotExist(err) {
						err := os.Mkdir("screen", 0755)
						if err != nil {
							log.Printf("创建screen目录失败: %v\n", err)
						} else {
							log.Println("已创建screen目录")
						}
					}
					if _, err = os.Stat("screen\\" + strconv.FormatUint(deviceId, 10)); os.IsNotExist(err) {
						err = os.Mkdir("screen\\"+strconv.FormatUint(deviceId, 10), 0755)
						if err != nil {
							logs.Info("创建%d目录失败: %v\n", deviceId, err)
						} else {
							logs.Info("已创建%d目录", deviceId)
						}
					}
					timestamp := time.Now().Format("20060102_150405")
					filePath := fmt.Sprintf("screen\\"+strconv.FormatUint(deviceId, 10)+"\\%s.png", timestamp)
					err = os.WriteFile(filePath, jpg, 0644)
					if err != nil {
						logs.Info("保存图片失败%v\n", deviceId, err)
						return
					}

				}
			}
		}
	} else {
		logs.Error("公共rtc收到的图片数据小于16长度")
	}

}
func (s *DeviceInfo) Rtc切换输入法(输入法包名 string) {
	//"com.google.android.inputmethod.latin/com.android.inputmethod.latin.LatinIME"
	if s.Online&2 == 0 {
		logs.Error("<手机离线>")
		return
	}

	//	logs.Info("控制端deviceId=%d，Rtc开启H264投屏", s.DeviceId)
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncChangeInput, atomic.AddUint32(&S.MsgSeq, 1))
	type InputChange struct {
		Ime string `json:"imeId" msgpack:"imeId"`
	}

	streamReq := InputChange{
		Ime: 输入法包名,
	}

	if err := msg.Marshal(&streamReq); err != nil {
		logs.Error("控制窗口[%d]序列化切换输入法请求消息失败,%s", s.DeviceId, err.Error())
		return
	}
	if s.RtcConn == nil {
		logs.Error("控制窗口[%d]Rtc连接未建立,无法发送切换输入法请求消息", s.DeviceId)
		return
	}
	if err := s.RtcConn.SendMsgpack(msg, s.DeviceId); err != nil {
		logs.Error("控制窗口[%d]发送切换输入法请求消息失败,%s", s.DeviceId, err.Error())
		return
	} else {
		logs.Info("控制窗口[%d]已发送切换输入法请求: %v", string(msg.DataMsgpack))
	}

}
func (s *DeviceInfo) Rtc切换摄像头(id int) {
	if s.Online&2 == 0 {
		logs.Error("<手机离线>")
		return
	}
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncChangSwitch, atomic.AddUint32(&S.MsgSeq, 1))

	if id == 0 {
		var streamReq Setting0
		streamReq.Type = "setting"
		streamReq.Params.SwitchBack = nil
		if err := msg.Marshal(&streamReq); err != nil {
			logs.Error("控制窗口[%d]序列化Rtc切换摄像头请求消息失败,%s", s.DeviceId, err.Error())
			return
		}
	} else {
		var streamReq Setting1
		streamReq.Type = "setting"
		streamReq.Params.SwitchFront = nil
		if err := msg.Marshal(&streamReq); err != nil {
			logs.Error("控制窗口[%d]序列化Rtc切换摄像头请求消息失败,%s", s.DeviceId, err.Error())
			return
		}
	}
	if s.RtcConn == nil {
		logs.Error("控制窗口[%d]Rtc连接未建立,无法发送Rtc切换摄像头请求消息", s.DeviceId)
		return
	}
	if err := s.RtcConn.SendMsgpack(msg, s.DeviceId); err != nil {
		logs.Error("控制窗口[%d]发送Rtc切换摄像头请求消息失败,%s", s.DeviceId, err.Error())
		return
	} else {
		logs.Info("控制窗口[%d]已发送Rtc切换摄像头请求: %v", string(msg.DataMsgpack))
	}
}
