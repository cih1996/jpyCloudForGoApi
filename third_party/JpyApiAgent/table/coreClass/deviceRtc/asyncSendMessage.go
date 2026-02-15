package deviceRtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"sync/atomic"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
)

// AsyncSendMessage 异步方式向设备发送指令
//
// msgType 指令类型 对应文档参数：f
// msgBody 指令参数 对应文档参数：data
// returnMsg 是否返回指令执行结果 对应文档参数：req
// logMsg 日志信息  对应文档参数：功能名称
func (s *TypeInfo) AsyncSendMessage(msgType uint16, msgBody interface{}, req bool, logMsg string) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, msgType, atomic.AddUint32(&s.Seq, 1))
	msg.Req = req
	if msgBody != nil {
		if err := msg.Marshal(msgBody); err != nil {
			logs.Error("设备[%d]%s数据构造失败,%s", s.DeviceId, logMsg, err.Error())
			return
		}
	}
	if s.Rtc != nil {
		if err := s.Rtc.SendMsgpack(msg, 0); err != nil {
			logs.Error("设备[%d]%s发送指令消息失败,%s", s.DeviceId, logMsg, err.Error())
			return
		}
	}
	return
}

// AsyncStartVideo 开启H264串流
// reConnect Rtc重连后是否自动恢复串流，0=自动恢复，1=手动恢复
func (s *TypeInfo) AsyncStartVideo(reConnect int32) {
	s.VideoStatus.ReConnect = reConnect
	s.AsyncSendMessage(public.FuncStartVideo, &publicStruct.H264StreamRequest{
		FPS:     30,
		Bitrate: 4000000,
		Quality: 10,
		Width:   s.MiddleAgentDevice.Width / 2,
	}, true, "开启H264串流")
}

// AsyncStopVideo 停止H264串流
func (s *TypeInfo) AsyncStopVideo() {
	s.AsyncSendMessage(public.FuncStopVideo, nil, true, "停止H264串流")
}

// AsyncStartAudio 开启音频
// reConnect Rtc重连后是否自动恢复音频，0=自动恢复，1=手动恢复
func (s *TypeInfo) AsyncStartAudio(reConnect int32) {
	s.AudioStatus.ReConnect = reConnect
	s.AsyncSendMessage(public.FuncStartAudio, &publicStruct.AudioStreamRequest{
		SampleRate:   48000,
		AudioBitRate: 128000,
	}, true, "开启音频")
}

// AsyncStopAudio 停止音频
func (s *TypeInfo) AsyncStopAudio() {
	s.AsyncSendMessage(public.FuncStopAudio, nil, true, "停止音频")
}

// AsyncTouch 触屏操作
func (s *TypeInfo) AsyncTouch(touchType int, x int, y int, offset int, pressure int, id int) {
	s.AsyncSendMessage(public.FuncTouch, &publicStruct.TouchRequest{
		Type:     touchType,
		X:        x,
		Y:        y,
		Offset:   offset,
		Pressure: pressure,
		Id:       id,
	}, false, "触屏操作")
}

// AsyncScroll 鼠标中键操作
func (s *TypeInfo) AsyncScroll(upOrDown int, x int, y int) {
	s.AsyncSendMessage(public.FuncScroll, &publicStruct.ScrollRequest{
		UpOrDown: upOrDown,
		X:        x,
		Y:        y,
	}, false, "鼠标中键操作")
}

// AsyncKeyboard 键盘操作
func (s *TypeInfo) AsyncKeyboard(keyCode int, action int) {
	s.AsyncSendMessage(public.FuncKey, &publicStruct.KeyBoardRequest{
		KeyCode: keyCode,
		Action:  action,
	}, false, "键盘操作")
}
