package coreClass

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/BufferRTC"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/netclient/netclient"
	"github.com/ghp3000/public"
	"github.com/goccy/go-json"
	"github.com/vmihailenco/msgpack/v5"
)

// MiddleRtcConnect 打洞连接中间件
func (c *Core) MiddleRtcConnect(tokenInfo RtcToken) {
	mid, err := textToUint64Auto(tokenInfo.Host)
	if err != nil {
		logs.Error("[MiddleRtcClient] midId转换失败,err=%s", err.Error())
		return
	}
	logs.Info("[MiddleRtcClient],middleID=%d,GuestUrl=%s,Token=%s", mid, tokenInfo.GuestUrl, tokenInfo.Token)
	rtc, err := BufferRTC.New(mid, tokenInfo.GuestUrl, tokenInfo.Token, true, bufferPool.Buffer, c.公共Rtc连接成功, c.公共Rtc连接断开, c.onRtcData)
	if err != nil {
		logs.Error("[MiddleRtcClient] err:%s,TokenInfo=%v", err.Error(), tokenInfo)
		return
	}
	logs.Info("发起公共中间件连接[MiddleRtcClient],mid=%d,user=%d", rtc.Extra(), tokenInfo.UserId)
}

func (c *Core) 心跳处理流程(_ *bufferPool.Packet, conn netclient.NetClient) {
	if err := conn.SendPong(); err != nil {
		err = conn.Close()
		if err != nil {
			logs.Error("[MiddleRtcClient]关闭连接错误,err=%s", err.Error())
		}
	}
	if conn.Extra().(uint64) == 21 {
		//logs.Info("[MiddleRtcClient]<TypePing处理>,id=%d", conn.Extra().(uint64))
	}

}
func (c *Core) onRtcData(a *bufferPool.Packet, conn netclient.NetClient) bool {
	logs.Info("onRtcData called, Type=%d", a.Type())
	var id uint64
	if err := a.UnmarshalHeader(&id); err != nil {
		logs.Error("[MiddleRtcClient]解析包头内容失败")
	} else {
		//logs.Info("收到设备编号是:%d的数据,deviceid为:%d", deviceId)
	}
	typ := a.Type()
	switch typ {
	case bufferPool.TypePing:
		go c.心跳处理流程(a, conn)
		break
	case bufferPool.TypePong:
		//		logs.Info("[MiddleRtcClient]中间件[%d]收到心跳", conn.Extra().(uint64))
		MidRtc, ok := c.MidGetConn(conn.Extra().(uint64))
		if ok {
			atomic.StoreInt32(&MidRtc.Heartbeat, 1)
		}
		break
	case bufferPool.TypeMsgpack:
		var msg public.Message
		if err := a.Unmarshal(&msg); err != nil {
			logs.Error("[MiddleRtcClient]收到的msgpack数据反序列化失败,", err)
		}
		msg.Type = typ
		logs.Info("onRtcData received Msgpack: F=%d, Seq=%d", msg.F, msg.Seq)
		//否则按异步处理
		go c.公共Rtc收到消息(&msg, id, conn)
	case bufferPool.TypeJson:
		var msg public.Message
		if err := a.Unmarshal(&msg); err != nil {
			logs.Error("[MiddleRtcClient]收到的json数据反序列化失败,", err)
		}
		msg.Type = typ
		go c.公共Rtc收到消息(&msg, id, conn)
	case bufferPool.TypeTestDelayResponse:
		var v DelayRequest
		if err := a.Unmarshal(&v); err != nil {
			logs.Error("[MiddleRtcClient]返回的消息反序列化失败,", err)
			return true
		}
		logs.Info("[MiddleRtcClient]异步测速的消息回来了,延迟=%d微秒", time.Now().UnixMicro()-v.Timestamp)

	default:
		logs.Error("[MiddleRtcClient]未处理的类型: %d%s", typ)
	}
	return true
}
func (c *Core) MiddleRtc收到设备shell命令返回(deviceId uint64, msg *public.Message) {
	var res shellcmd
	err := msg.Unmarshal(&res)
	if err != nil {
		logs.Error("[MiddleRtcClient]收到设备shell命令返回发生错误,deviceId=%d,msg=%s,err=%s", deviceId, string(msg.DataMsgpack), err.Error())
		return
	}
	logs.Info("[MiddleRtcClient]收到设备shell命令返回,deviceId=%d,msg=%s", deviceId, res.Shell, msg.Seq)
	c.ShellRec.Store(deviceId, res.Shell)

}
func (c *Core) 公共Rtc收到消息(msg *public.Message, Id uint64, conn netclient.NetClient) {
	logs.Info("[MiddleRtcClient]收到消息 F=%d, Seq=%d, Id=%d, Len=%d", msg.F, msg.Seq, Id, len(msg.DataMsgpack))

	// Check for synchronous callbacks
	if val, ok := c.SyncCallbacks.Load(msg.Seq); ok {
		if ch, ok := val.(chan interface{}); ok {
			select {
			case ch <- msg:
			default:
				logs.Warn("SyncCallback channel full for Seq: %d", msg.Seq)
			}
			return
		}
	}

	// Check for synchronous callbacks by DeviceID + F (for cases where Seq is not echoed)
	if msg.Seq == 0 {
		deviceId := GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id))
		key := fmt.Sprintf("%d-%d", deviceId, msg.F)
		if val, ok := c.SyncCallbacksByDeviceF.Load(key); ok {
			if ch, ok := val.(chan interface{}); ok {
				select {
				case ch <- msg:
				default:
					logs.Warn("SyncCallbackByDeviceF channel full for Key: %s", key)
				}
				// We don't return here because we might also want to process it via other handlers if needed,
				// but typically if it's a sync response, we are done.
				return
			}
		}
	}

	logs.Info("[MiddleRtcClient]收到消息收到消息,deviceId=%d,msg=%v", Id, string(msg.DataMsgpack))
	switch msg.F {
	// case public.FuncTestDelay:
	// 	var v DelayRequest
	// 	if err := msg.Unmarshal(&v); err != nil {
	// 		logs.Error("控制窗口rtc返回的消息反序列化失败,", err)
	// 	}
	// case public.FuncScreenChange:
	// 	c.MiddleRtc收到屏幕旋转事件(GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), msg)
	// case public.FuncImg:
	// 	//fmt.Println("收到屏幕墙图片", len(msg.DataMsgpack))
	// 	deviceId := GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id))
	// 	go c.MiddleRtc收到屏幕墙图片(deviceId, msg)
	// case public.FuncDevices:
	// 	c.MiddleRtc收到设备列表信息(msg, conn)
	// case public.FuncOnlineList:
	// 	c.MiddleRtc收到上线离线信息(conn, msg, Id)
	// case public.FuncDevice: //一台
	// 	deviceId := GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id))
	// 	c.MiddleRtc收到单台设备信息(msg, deviceId, conn)
	// case public.FuncWakeUPAways: //常亮
	// 	//		logs.Info("[MiddleRtcClient]设备常亮处理成功，设备Id=%d", Id)
	// 	midId := conn.Extra().(uint64)
	// 	MidRtc, ok := c.MidGetConn(midId)
	// 	if ok {
	// 		atomic.StoreInt32(&MidRtc.Let常亮, 1)
	// 	}
	// case public.FuncFileDownload:
	// 	c.MiddleRtc收到设备下载状态信息(GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), msg)
	// case public.FuncGetAppList:
	// 	c.MiddleRtc收到设备app列表信息(GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), msg)
	// case public.FuncCMDWithResult:
	// 	c.MiddleRtc收到设备shell命令返回(GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), msg)
	default:
		//logs.Error("[MiddleRtcClient]未处理的函数 F=%d,设备Id=%d,msg=%s", msg.F, Id, string(msg.DataMsgpack))
		var a interface{}
		err := msg.Unmarshal(&a)
		if err != nil {
			logs.Error("[MiddleRtcClient]<未解析的应用类型解析错误>,err=%s", err.Error())
			return
		}
		logs.Error("%v", string(msg.DataMsgpack))

		jsonData, err := json.Marshal(a)
		if err != nil {
			logs.Error("[MiddleRtcClient]转换JSON失败: %v", err)
			return
		}
		logs.Info("[MiddleRtcClient]未处理的函数 F=[%d],设备deivceId=[%d]， %s..", msg.F, GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), uint8(Id)), string(jsonData))
	}
}

// MiddleRtcSendGenericCommand sends a generic command to devices via a specific middleware proxy.
func (c *Core) MiddleRtcSendGenericCommand(proxyId uint64, deviceIds []uint64, cmd GenericCommand, isSync bool, timeout time.Duration) (interface{}, error) {
	if len(deviceIds) == 0 {
		return nil, fmt.Errorf("no device ids provided")
	}

	// Always generate a new Seq to ensure uniqueness for sync tracking
	seq := atomic.AddUint32(&c.MsgSeq, 1)
	cmd.Seq = seq

	var ch chan interface{}
	if isSync {
		// Buffered channel to hold responses from all devices
		ch = make(chan interface{}, len(deviceIds))
		c.SyncCallbacks.Store(seq, ch)
		defer c.SyncCallbacks.Delete(seq)

		// Also register by DeviceID + F for devices that return Seq=0
		for _, devId := range deviceIds {
			key := fmt.Sprintf("%d-%d", devId, cmd.F)
			c.SyncCallbacksByDeviceF.Store(key, ch)
			defer c.SyncCallbacksByDeviceF.Delete(key)
		}
	}

	// Get connection by ProxyID (Method 2: Middleware connection)
	midRtc, ok := c.MidGetConn(proxyId)
	if !ok || midRtc.Conn == nil {
		return nil, fmt.Errorf("middleware proxy %d not connected", proxyId)
	}

	successCount := 0
	for _, devId := range deviceIds {
		// Method 2: Header ID should be the Device ID, sent via Proxy Connection
		headerIds := []uint64{devId}
		msg := public.NewMessage(bufferPool.TypeMsgpack, cmd.F, cmd.Seq)
		if err := msg.Marshal(&cmd.Data); err != nil {
			logs.Error("Marshal command data failed for device %d: %v", devId, err)
			continue
		}
		msg.Req = cmd.Req

		pkt := bufferPool.Get()
		// Important: We send to the Proxy Connection, but address the Device ID in the header
		if err := pkt.Write3(bufferPool.TypeMsgpack, headerIds, &msg); err != nil {
			pkt.Put()
			logs.Error("Write packet failed for device %d: %v", devId, err)
			continue
		}

		logs.Info("Sending packet to device %d via proxy %d, F=%d, Seq=%d", devId, proxyId, cmd.F, cmd.Seq)
		if err := midRtc.Conn.SendPacket(pkt); err != nil {
			// pkt.Put()
			logs.Error("Send packet failed for device %d: %v", devId, err)
			continue
		}
		logs.Info("Packet sent to device %d via proxy %d, F=%d, Seq=%d", devId, proxyId, cmd.F, cmd.Seq)
		// pkt.Put() // Commented out to prevent potential race condition if SendPacket is async
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("failed to send to any device (checked %d devices)", len(deviceIds))
	}

	if isSync {
		// Collect results
		results := make([]interface{}, 0, successCount)
		// We use a loop with select to gather responses
		// We stop if we get all expected responses or timeout

		// Note: We might get fewer responses than successCount if some fail silently on the other end
		// So we rely on timeout to finish if not all respond.

		endTime := time.Now().Add(timeout)

		for len(results) < successCount {
			remaining := time.Until(endTime)
			if remaining <= 0 {
				break
			}

			select {
			case res := <-ch:
				results = append(results, res)
			case <-time.After(remaining):
				// Timeout will be handled by the loop condition on next iteration or break
				goto DONE
			}
		}
	DONE:
		if len(results) == 0 {
			return nil, fmt.Errorf("timeout waiting for response")
		}

		// If we targeted exactly one device and got one result, return it directly
		if len(deviceIds) == 1 && len(results) > 0 {
			return results[0], nil
		}

		// Otherwise return the list of results
		return results, nil
	}

	return cmd.Seq, nil
}

func (c *Core) MiddleRtc收到屏幕旋转事件(deviceId uint64, msg *public.Message) {
	logs.Info("[MiddleRtcClient]收到屏幕旋转事件,设备编号:%d,%s", deviceId, msg.DataMsgpack)
	//{"orientation":1}

}
func (c *Core) MiddleRtc收到设备app列表信息(_ uint64, msg *public.Message) {

}
func (c *Core) MiddleRtc收到设备下载状态信息(deviceId uint64, msg *public.Message) {
	//{"beginTime":1745476434,"id":46,"install":false,"lastTime":1745476437,"md5":null,"path":null,"process":0.7523638585835919,"status":1,"url":"https://cos.htsystem.cn/acc-1312687568/cb16919b0d71cb2d8352fbb9f3e6cc911661ff6165b706b8fe0b8848f2d5b326.apk"}
}
func (c *Core) MiddleRtc收到设备列表信息(msg *public.Message, conn netclient.NetClient) {
	if len(msg.DataMsgpack) == 0 {
		logs.Warn("[MiddleRtcClient]收到设备列表信息为空")
		midId := conn.Extra().(uint64)
		MidRtc, ok := c.MidGetConn(midId)
		if ok {
			atomic.StoreInt32(&MidRtc.Let设备列表, 1)
		}
		return
	}
	var devices []RtcDeviceInfo
	err := msgpack.Unmarshal(msg.DataMsgpack, &devices)
	if err != nil {
		logs.Error("[MiddleRtcClient]解析设备信息失败: %v, len=%d, data=%x", err, len(msg.DataMsgpack), msg.DataMsgpack)
		return
	}
	for _, device := range devices {
		deviceId := GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), device.Seat)
		ph, ok := c.DidGetPhoneInfo(deviceId)
		if ok {
			ph.DeviceWidth = device.Width
			ph.DeviceHeight = device.Height
			ph.RtcDevice.AndroidVersion = device.AndroidVersion
			ph.RtcDevice.CPU = device.CPU
			ph.RtcDevice.DiskSize = device.DiskSize
			ph.RtcDevice.Height = device.Height
			ph.RtcDevice.Memory = device.Memory
			ph.RtcDevice.Model = device.Model
			ph.RtcDevice.OsVersion = device.OsVersion
			ph.RtcDevice.Seat = device.Seat
			ph.RtcDevice.UUID = device.UUID
			ph.RtcDevice.Width = device.Width
			c.DidSaveDeviceInfo(deviceId, ph)
		}
	}

	midId := conn.Extra().(uint64)
	MidRtc, ok := c.MidGetConn(midId)
	if ok {
		atomic.StoreInt32(&MidRtc.Let设备列表, 1)
	}

	jsonData, err := json.Marshal(devices)
	logs.Info("MiddleRtc收到当前中间件[%d]，索引设备列表信息:%s,%v", conn.Extra().(uint64), string(jsonData), err)
}
func (c *Core) 公共Rtc连接成功(conn netclient.NetClient) {
	//返回成功，建立连接时候存
	go c.Middlertc线程调用连接成功(conn)

}
func (c *Core) Middlertc线程调用连接成功(conn netclient.NetClient) {
	logs.Info("[MiddleRtcClient]公共中间件连接成功,mid=%d", conn.Extra().(uint64))
	middleId := conn.Extra().(uint64)
	var MidRtc MiddleRtc
	MidRtc.middleId = middleId
	MidRtc.Conn = conn
	atomic.StoreInt32(&MidRtc.Heartbeat, 1)
	c.MidSaveConn(middleId, &MidRtc)

	// c.MiddleRtc拉取指定中间件所连接的设备列表(MidRtc.Conn)
	// c.MiddleRtc拉取在线列表(MidRtc.Conn, middleId)
	// c.MiddleRtc拉取指定中间件所连接的设备信息(MidRtc.Conn, middleId)
	// go c.MiddleRtc线程判断必要信息是否发送(middleId)
	client, ok := conn.(*BufferRTC.RtcClient)
	if ok {
		logs.Info("当前打洞的工作模式=========================================================================为:%s", client.Client.TunnelMode())
	}

}
func (c *Core) MiddleRtc线程判断必要信息是否发送(middleId uint64) {
	MidRtc, ok := c.MidGetConn(middleId)
	time.Sleep(2 * time.Second)
	if ok {
		for {
			if atomic.LoadInt32(&MidRtc.Let设备列表) == 1 && atomic.LoadInt32(&MidRtc.Let在线列表) == 1 && atomic.LoadInt32(&MidRtc.Let单设备信息) == 1 {
				logs.Info("[MiddleRtcClient]线程判断必要信息是否发送,中间件[%d]已经全部获取到", middleId)
				return
			}
			if atomic.LoadInt32(&MidRtc.Let设备列表) == 0 && MidRtc.Conn != nil {
				c.MiddleRtc拉取指定中间件所连接的设备列表(MidRtc.Conn)
			}
			if atomic.LoadInt32(&MidRtc.Let在线列表) == 0 && MidRtc.Conn != nil {
				c.MiddleRtc拉取在线列表(MidRtc.Conn, middleId)
			}
			if atomic.LoadInt32(&MidRtc.Let单设备信息) == 0 && MidRtc.Conn != nil {
				c.MiddleRtc拉取指定中间件所连接的设备信息(MidRtc.Conn, middleId)
			}
			time.Sleep(2 * time.Second)
		}
	}
}
func (c *Core) 公共Rtc连接断开(conn netclient.NetClient) {
	c.Middlertc线程调用连接断开(conn)
}
func (c *Core) Middlertc线程调用连接断开(conn netclient.NetClient) {
	logs.Info("[MiddleRtcClient]公共中间件断开入口,mid=%d", conn.Extra().(uint64))
	mid := conn.Extra().(uint64)
	MidRtc, ok := c.MidGetConn(mid)
	if ok {
		if MidRtc.Conn != nil {
			err := MidRtc.Conn.Close()
			if err != nil {
				logs.Info("[MiddleRtcClient]公共中间件断开补充操作,mid=%d", conn.Extra().(uint64))
			}
		}
	}

	c.MidDelConn(mid)
	if c.IsClose == true {
		logs.Info("[MiddleRtcClient]公共中间件连接断开，mid=%d,Pc端主动关闭不需要重连", conn.Extra().(uint64))
		return
	}

	if c.ReconnectCallback != nil {
		logs.Info("[MiddleRtcClient] Triggering reconnection for proxy %d", mid)
		go c.ReconnectCallback(mid)
	}

	var mids []uint64
	mids = append(mids, mid)
	time.Sleep(9 * time.Second)

}
func (c *Core) MiddleRtc收到单台设备信息(msg *public.Message, deviceId uint64, _ netclient.NetClient) {
	////{f:4,data:{"sysVersion":"android14","country":"United States(US)","appVersion":"","orientation":1,"os":"android","timezone":"Greenwich Mean Time","sysPer":1,"uuid":"aaaaa","seat":1,"osVersion":"Pixel_4a5g_android_14_20241124_v1.0","androidVersion":"14","vendor":"OPPO PHT110","width":1080,"signalMode":"","self":"1.1","model":"PHT110","location":"{}","lang":"en","brand":"OPPO","height":2520},req:false,seq:1,code:0,msg:"xxx"}

	MidId := GetMiddleIdFromDeviceId(deviceId)
	MidRtc, ok := c.MidGetConn(MidId)
	if ok {
		atomic.StoreInt32(&MidRtc.Let单设备信息, 1)
	}

	var device Phone单台信息
	err := msgpack.Unmarshal(msg.DataMsgpack, &device)
	if err != nil {
		logs.Error("[MiddleRtcClient]解析设备信息失败: %v,%v,%d", err, msg, deviceId)
		return
	}
	//{"latitude":33.990962,"longitude":119.031425}
	type latlng struct {
		Latitude  float64 `json:"latitude" msgpack:"latitude"`
		Longitude float64 `json:"longitude" msgpack:"longitude"`
	}
	var LatLng latlng

	ph, ok := c.DidGetPhoneInfo(deviceId)
	if ok {
		ph.DeviceWidth = device.Width
		ph.DeviceHeight = device.Height
		ph.Orientation = device.Orientation

		ph.RtcDevice.SysVersion = device.SysVersion
		ph.RtcDevice.Country = device.Country
		ph.RtcDevice.AppVersion = device.AppVersion

		ph.RtcDevice.Os = device.Os
		ph.RtcDevice.Timezone = device.Timezone
		ph.RtcDevice.SysPer = device.SysPer
		ph.RtcDevice.UUID = device.Uuid
		ph.RtcDevice.OsVersion = device.OsVersion
		//		logs.Info("[MiddleRtcClient]收到设备列表信息,设备编号:%d,%s", deviceId, ph.RtcDevice.OsVersion)
		ph.RtcDevice.Vendor = device.Vendor
		ph.RtcDevice.SignalMode = device.SignalMode
		ph.RtcDevice.Self = device.Self
		if device.Location == "" {
			device.Location = "{}"
		}
		err = json.Unmarshal([]byte(device.Location), &LatLng)
		if err != nil {
			logs.Error("[MiddleRtcClient]<解析定位失败>: %v", err)
		}

		ph.RtcDevice.Location = fmt.Sprintf("经度:%f,纬度:%f", LatLng.Longitude, LatLng.Latitude)
		ph.RtcDevice.Lang = device.Lang
		ph.RtcDevice.Brand = device.Brand
		c.DidSaveDeviceInfo(deviceId, ph)
	}

	//测试代码
	//phone, ok := c.DidGetPhoneInfo(deviceId)
	//if ok {
	//	logs.Info("检测设备信息:%s", phone.RtcDevice.OsVersion)
	//}

	//jsonData, err := json.Marshal(device)
	//	logs.Info("[MiddleRtcClient]Rtc收到单台设备信息: %v", string(jsonData))
	if c.Callback设备列表回调 != nil {
		c.Callback设备列表回调(deviceId)
	}
} //
func (c *Core) MiddleRtc收到屏幕墙图片(deviceId uint64, msg *public.Message) {
	//	logs.Info("收到获取图片事件,设备编号:%d", deviceId, msg.F, msg.DataMsgpack, msg.Code)
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
		ph, ok := c.DidGetPhoneInfo(deviceId)
		if ok {
			ph.Picture = ph.Picture[:0]
			if c.Rtc屏幕墙取图方式 == 2 {
				jpg, err := c.webp转jpg(imgData, 90)
				if err != nil {
					return
				}
				ph.Picture = jpg
			} else {
				ph.Picture = imgData
			}

			角度 := atomic.LoadInt32(&S.S屏幕墙角度orientation)
			switch 角度 {
			case 0:
				break
			case 1:
				ph.Picture = Rotate90Bytes(ph.Picture)
				break
			case 2:
				ph.Picture = Rotate180Bytes(ph.Picture)
				break
			case 3:
				ph.Picture = Rotate270Bytes(ph.Picture)
				break
			}
			c.DidSaveDeviceInfo(deviceId, ph)
			if c.CallbackWallPicture != nil {
				c.CallbackWallPicture(int(deviceId))
			}
		}
	} else {
		logs.Error("公共rtc收到的图片数据小于16长度")
	}
}

func (c *Core) MiddleRtc收到上线离线信息(conn netclient.NetClient, msg *public.Message, _ uint64) {
	if msg.Code != 0 {
		logs.Error("公共rtc获取离线信息失败,msg=%s", msg.DataMsgpack)
		return
	}
	// 定义离线信息结构体
	type OnlineInfo struct {
		Online int32  `msgpack:"online" json:"online"` // 在线状态 0=离线 1=在线
		Seat   uint8  `msgpack:"seat" json:"seat"`     // 座位号
		Ip     string `msgpack:"ip" json:"ip"`         // 中间件ID
	}
	// 解析离线信息列表
	var onlineInfoList []OnlineInfo
	if err := msg.Unmarshal(&onlineInfoList); err != nil {
		logs.Error("公共rtc收到的离线信息数据反序列化失败: %v", err)
		return
	}

	for _, onlinemsg := range onlineInfoList {
		deviceId := GetDeviceIdFromMiddleIdAndSeat(conn.Extra().(uint64), onlinemsg.Seat)
		for n := 0; n < len(c.Devices); n++ {
			if c.Devices[n].DeviceId == deviceId {
				c.Devices[n].OnLine = onlinemsg.Online
				c.Devices[n].Ip = onlinemsg.Ip
				//logs.Info("MiddleRtc收到上线离线信息,deviceId=%d,online=%d", deviceId, onlinemsg.Online)
				break
			}

		}

		ph, ok := c.DidGetPhoneInfo(deviceId)
		if ok {
			ph.Online = onlinemsg.Online
			ph.ReqDevice.DeviceIp = onlinemsg.Ip
			c.DidSaveDeviceInfo(deviceId, ph)
			if onlinemsg.Online&2 > 0 {
				if atomic.LoadInt32(&ph.H264IsOpen) == 1 && atomic.LoadInt32(&ph.WindowIsOpen) == 1 {
					ph.Rtc开启H264串流()
				}
				c.MiddleRtc设置单个常量(deviceId)
				c.MiddleRtc拉取指定中间件设备信息(deviceId)
			} else {
				//				logs.Error("设备离线,deviceId=%d", deviceId)

			}

		}
	}

	midId := conn.Extra().(uint64)
	MidRtc, ok := c.MidGetConn(midId)
	if ok {
		atomic.StoreInt32(&MidRtc.Let在线列表, 1)
	}

	jsonData, err := json.Marshal(onlineInfoList)
	logs.Info("MiddleRtc收到上线离线信息mid=[%d],%s,%v", conn.Extra().(uint64), string(jsonData), err)
	c.CallbackSetWall()

}
func (c *Core) MiddleRtc拉取指定中间件所连接的设备列表(conn netclient.NetClient) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncDevices, atomic.AddUint32(&c.MsgSeq, 1))
	if conn != nil {
		if err := conn.SendMsgpack(msg, 0); err != nil {
			logs.Error("公共rtc发送图片请求消息失败,%s", err.Error())
			return
		}
	}
	logs.Info("公共rtc拉取指定中间件所连接的设备列表")
	return
}
func (c *Core) MiddleRtc拉取指定中间件设备信息(deviceId uint64) {
	var deviceIds []uint64
	var MidRtc *MiddleRtc
	deviceIds = append(deviceIds, deviceId)
	ph, ok := c.DidGetPhoneInfo(deviceId)
	if ok {
		MidRtc, ok = c.MidGetConn(ph.ReqDevice.ProxyId)
		if ok {
			msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncDevice, atomic.AddUint32(&c.MsgSeq, 1))
			pkt := bufferPool.Get()
			defer pkt.Put()
			err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
			if err != nil {
				return
			}
			if MidRtc.Conn != nil {
				if err = MidRtc.Conn.SendPacket(pkt); err != nil {
					logs.Error("公共rtc发送图片请求消息失败,%s", err.Error())
					return
				}
			}
		}
	}

}
func (c *Core) MiddleRtc拉取指定中间件所连接的设备信息(conn netclient.NetClient, midId uint64) {
	//{f:4,data:null,req:true,seq:1}
	var deviceIds []uint64
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncDevice, atomic.AddUint32(&c.MsgSeq, 1))

	pkt := bufferPool.Get()
	defer pkt.Put()
	for _, device := range c.Devices {
		if device.ProxyId == midId {
			deviceIds = append(deviceIds, device.DeviceId)
			if len(deviceIds) == 30 {
				err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
				if err != nil {
					return
				}
				if conn != nil {
					if err = conn.SendPacket(pkt); err != nil {
						logs.Error("公共rtc发送图片请求消息失败,%s", err.Error())
						return
					}
					deviceIds = deviceIds[:0]
				}
			}
		}
	}
	if len(deviceIds) > 0 {
		err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
		if err != nil {
			return
		}

		if conn != nil {
			if err = conn.SendPacket(pkt); err != nil {
				logs.Error("公共rtc发送图片请求消息失败,%s", err.Error())
				return
			}
		}

	}
	return
}
func (c *Core) MiddleRtc批量设置常亮(conn netclient.NetClient, midId uint64) {
	//{"f": 298,"data": {"state": "state", "timeLong":31536000}, "req": true,"seq":1 }
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncWakeUPAways, atomic.AddUint32(&c.MsgSeq, 1))
	imgReq := WakeUpAways{
		State:    "on",
		TimeLong: 31536000,
	}
	if err := msg.Marshal(&imgReq); err != nil {
		logs.Error("MiddleRtc批量设置常亮,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	var i int
	var deviceIds []uint64
	for _, device := range c.Devices {
		if device.ProxyId == midId {
			deviceIds = append(deviceIds, device.DeviceId)
			if len(deviceIds) == 30 {
				err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
				if err != nil {
					return
				}
				if conn != nil {
					if err = conn.SendPacket(pkt); err != nil {
						logs.Error("MiddleRtc批量设置常亮,%s", err.Error())
						return
					}
				}

				deviceIds = deviceIds[:0]
				i++
				logs.Info("MiddleRtc批量设置常亮次数:%d", i)
			}

		}
	}
	if len(deviceIds) > 0 {
		err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
		if err != nil {
			return
		}
		if conn != nil {
			if err = conn.SendPacket(pkt); err != nil {
				logs.Error("MiddleRtc批量设置常亮,%s", err.Error())
				return
			}
		}
		i++
		logs.Info("MiddleRtc批量设置常亮次数:%d", i)
	}
	return
}
func (c *Core) MiddleRtc根据中间件切换输入法(_ netclient.NetClient, midId uint64) {
	var deviceIds []uint64
	for _, device := range c.Devices {
		if device.ProxyId == midId {
			deviceIds = append(deviceIds, device.DeviceId)
		}
	}
	c.MiddleRtc批量锁定输入法(deviceIds, c.输入法包名)
}
func (c *Core) MiddleRtc设置单个常量(diviceId uint64) {
	mid := GetMiddleIdFromDeviceId(diviceId)
	Mid, ok := c.MidGetConn(mid)
	if ok {
		var deviceIds []uint64
		deviceIds = append(deviceIds, diviceId)
		msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncWakeUPAways, atomic.AddUint32(&c.MsgSeq, 1))
		imgReq := WakeUpAways{
			State:    "on",
			TimeLong: 31536000,
		}
		if err := msg.Marshal(&imgReq); err != nil {
			logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
			return
		}
		pkt := bufferPool.Get()
		defer pkt.Put()
		err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg)
		if err != nil {
			return
		}

		if Mid.Conn != nil {
			if err = Mid.Conn.SendPacket(pkt); err != nil {
				logs.Error("公共rtc发送图片请求消息失败,%s", err.Error())
				return
			}
		}

	}

	return
}

// MiddleRtcSendShellCommand 发送shell命令到指定设备
func (c *Core) MiddleRtcSendShellCommand(deviceId uint64, shell string) error {
	midId := GetMiddleIdFromDeviceId(deviceId)
	midRtc, ok := c.MidGetConn(midId)
	if !ok || midRtc.Conn == nil {
		return fmt.Errorf("middleware %d not connected", midId)
	}

	cmd := shellcmd{Shell: shell}
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncCMDWithResult, atomic.AddUint32(&c.MsgSeq, 1))
	if err := msg.Marshal(&cmd); err != nil {
		return fmt.Errorf("marshal shell command failed: %v", err)
	}

	pkt := bufferPool.Get()
	defer pkt.Put()

	deviceIds := []uint64{deviceId}
	if err := pkt.Write3(bufferPool.TypeMsgpack, deviceIds, &msg); err != nil {
		return fmt.Errorf("write packet failed: %v", err)
	}

	logs.Info("Sending shell command to device %d: %s", deviceId, shell)
	return midRtc.Conn.SendPacket(pkt)
}

func (c *Core) MiddleRtc拉取在线列表(conn netclient.NetClient, _ uint64) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncOnlineList, atomic.AddUint32(&c.MsgSeq, 1))
	if conn != nil {
		if err := conn.SendMsgpack(msg, 0); err != nil {
			logs.Error("发送图片请求消息失败,%s", err.Error())
			return
		}
	}

	return
}
func (c *Core) MiddleRtc批量取图(deviceIds []uint64, Width int) {
	//logs.Info("批量取图,deviceIds=%v,Width=%d", deviceIds, Width)
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	var phone *DeviceInfo
	for _, mid := range mids {
		var sendDeviceIds []uint64
		for _, deviceId := range deviceIds {
			phone, ok = S.DidGetPhoneInfo(deviceId)
			if ok {
				if phone.Online&2 > 0 {
					if GetMiddleIdFromDeviceId(deviceId) == mid {
						sendDeviceIds = append(sendDeviceIds, deviceId)
						if len(sendDeviceIds) == 2 {
							MidRtc, ok = c.MidGetConn(mid)
							if ok {
								if sendDeviceIds != nil && MidRtc.Conn != nil {
									c.MiddleRtc根据中间件取图(MidRtc.Conn, sendDeviceIds, Width)
									time.Sleep(time.Millisecond * 20)
								}

							}
							sendDeviceIds = nil
						}

					}
				}
			}
		}
		MidRtc, ok = c.MidGetConn(mid)
		if ok {
			if sendDeviceIds != nil && MidRtc.Conn != nil {
				c.MiddleRtc根据中间件取图(MidRtc.Conn, sendDeviceIds, Width)
			}

		}
	}
}
func (c *Core) MiddleRtc根据中间件取图(conn netclient.NetClient, diviceIds []uint64, Width int) {
	//	logs.Info("批量取图,diviceIds=%v,Width=%d", diviceIds, Width)
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncImg, atomic.AddUint32(&c.MsgSeq, 1))

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
		Type:   c.Rtc屏幕墙取图方式,
	}
	if err := msg.Marshal(&imgReq); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//		logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}
	//	logs.Info("发送图片请求消息,msg", string(msg.DataMsgpack))
	pkt := bufferPool.Get()
	defer pkt.Put()
	err := pkt.Write3(bufferPool.TypeMsgpack, diviceIds, &msg)
	if err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s,", err.Error(), diviceIds)
		return
	} else {
		//		logs.Info("序列化图片请求消息成功,msg=%v", msg)
	}

	if conn != nil {
		if err = conn.SendPacket(pkt); err != nil {
			logs.Error("公共rtc发送图片请求消息失败,%s", err.Error())
			return
		} else {
			//		logs.Info("发送图片请求消息成功,msg=%v", msg)
		}
	}

	return
}
func (c *Core) MiddleRtc触屏操作(conn netclient.NetClient, deviceIds []uint64, touchType int, x int, y int, offset int, pressure int, id int) {
	//{f:4,data:[ {type:0, x: 400, y: 500,offset:10,pressure:1,id:1}],req:true,seq:1}
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncTouch, atomic.AddUint32(&c.MsgSeq, 1))
	imgReq := TouchRequest{
		Type:     touchType,
		X:        x,
		Y:        y,
		Offset:   offset,
		Pressure: pressure,
		Id:       id,
	}
	var imgReqs []TouchRequest
	imgReqs = append(imgReqs, imgReq)

	if err := msg.Marshal(&imgReqs); err != nil {
		logs.Error("MiddleRtc触屏操作请求消息失败,%s", err.Error())
		return
	} else {
		//		logs.Info("MiddleRtc触屏操作消息成功,msg=%v", imgReq)
	}
	//	logs.Info("MiddleRtc触屏操作请求消息,msg", msg.DataMsgpack)
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
	} else {
		logs.Error("发送触屏操作失败,conn is nil")
		return
	}

}
func (c *Core) MiddleRtc输入法输入文本(conn netclient.NetClient, deviceIds []uint64, text string) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncInputText, atomic.AddUint32(&c.MsgSeq, 1))
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
func (c *Core) MiddleRtc批量输入法输入文本(deviceIds []uint64, text string) {
	mids := c.GetAllMid()
	var dids []uint64
	for _, mid := range mids {
		var MidRtc *MiddleRtc
		var ok bool
		var phone *DeviceInfo
		MidRtc, ok = c.MidGetConn(mid)
		if ok {
			for _, deviceId := range deviceIds {
				phone, ok = c.DidGetPhoneInfo(deviceId)
				if ok == true {
					if phone.ReqDevice.ProxyId == mid {
						dids = append(dids, deviceId)
						if len(dids) == 30 {
							c.MiddleRtc输入法输入文本(MidRtc.Conn, dids, text)
							dids = nil
						}
					}
				}
			}
			c.MiddleRtc输入法输入文本(MidRtc.Conn, dids, text)
		}
	}
}
func (c *Core) MiddleRtc发送Shell命令(conn netclient.NetClient, deviceIds []uint64, shell string, isreturn bool) {
	var msg *public.Message
	if isreturn == true {
		msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncCMDWithResult, atomic.AddUint32(&c.MsgSeq, 1))
	} else {
		msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncCMD, atomic.AddUint32(&c.MsgSeq, 1))
	}
	msg.Req = true
	msg.Seq = atomic.AddUint32(&c.MsgSeq, 1)
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
			logs.Error("MiddleRtc发送Shell命令失败,%s", err.Error())
			return
		}
	}

}
func (c *Core) MiddleRtc批量发送Shell命令(deviceIds []uint64, shell string, isreturn bool) {
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		var sendDeviceIds []uint64
		if ok {
			for _, deviceId := range deviceIds {
				if GetMiddleIdFromDeviceId(deviceId) == mid {
					sendDeviceIds = append(sendDeviceIds, deviceId)
					if len(sendDeviceIds) == 5 {
						c.MiddleRtc发送Shell命令(MidRtc.Conn, sendDeviceIds, shell, isreturn)
						sendDeviceIds = sendDeviceIds[:0]
						time.Sleep(time.Millisecond * 50)
					}
				}
			}
			if len(sendDeviceIds) > 0 {
				c.MiddleRtc发送Shell命令(MidRtc.Conn, sendDeviceIds, shell, isreturn)
				sendDeviceIds = sendDeviceIds[:0]
			}
		}
	}
}

func (c *Core) MiddleRtc批量提权或取消(deviceIds []uint64, packageName string, 提权或取消 bool) {
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		var sendDeviceIds []uint64
		if ok {
			for _, deviceId := range deviceIds {
				if GetMiddleIdFromDeviceId(deviceId) == mid {
					sendDeviceIds = append(sendDeviceIds, deviceId)
					if len(sendDeviceIds) == 5 {
						c.MiddleRtcRoot提权或取消提权(MidRtc.Conn, sendDeviceIds, packageName, 提权或取消)
						sendDeviceIds = sendDeviceIds[:0]
						time.Sleep(time.Millisecond * 50)
					}
				}
			}
			if len(sendDeviceIds) > 0 {
				c.MiddleRtcRoot提权或取消提权(MidRtc.Conn, sendDeviceIds, packageName, 提权或取消)
				sendDeviceIds = sendDeviceIds[:0]
			}
		}
	}
}

func (c *Core) MiddleRtcRoot提权或取消提权(conn netclient.NetClient, deviceIds []uint64, packageName string, 提权或取消 bool) {
	var msg *public.Message
	if 提权或取消 == true {
		msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncSetRootApp, atomic.AddUint32(&c.MsgSeq, 1))
	} else {
		msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncUnSetRootApp, atomic.AddUint32(&c.MsgSeq, 1))
	}

	//{packageName:"xxxxx"}
	type rootapp struct {
		PackageName string `msgpack:"pkg"`
	}

	methodInputReq := rootapp{
		PackageName: packageName,
	}

	if err := msg.Marshal(&methodInputReq); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}

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

func (c *Core) MiddleRtc启动app(conn netclient.NetClient, deviceIds []uint64, packageName string) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncRunApp, atomic.AddUint32(&c.MsgSeq, 1))
	//{packageName:"xxxxx"}
	type runapp struct {
		PackageName string `msgpack:"packageName"`
	}

	methodInputReq := runapp{
		PackageName: packageName,
	}

	if err := msg.Marshal(&methodInputReq); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}

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
func (c *Core) MiddleRtc批量设备启动app(deviceIds []uint64, packageName string) {
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		var sendDeviceIds []uint64
		if ok {
			for _, deviceId := range deviceIds {
				if GetMiddleIdFromDeviceId(deviceId) == mid {
					sendDeviceIds = append(sendDeviceIds, deviceId)
					if len(sendDeviceIds) == 5 {
						c.MiddleRtc启动app(MidRtc.Conn, sendDeviceIds, packageName)
						sendDeviceIds = sendDeviceIds[:0]
						time.Sleep(time.Millisecond * 50)
					}
				}
			}
			if len(sendDeviceIds) > 0 {
				c.MiddleRtc启动app(MidRtc.Conn, sendDeviceIds, packageName)
				sendDeviceIds = sendDeviceIds[:0]
			}
		}
	}
}
func (c *Core) MiddleRtc发送键盘消息(conn netclient.NetClient, deviceIds []uint64, keyCode int, action int) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncKey, atomic.AddUint32(&c.MsgSeq, 1))
	keyboardReq := keyboard{
		KeyCode: keyCode,
		Action:  action,
	}

	if err := msg.Marshal(&keyboardReq); err != nil {
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
func (c *Core) MiddleRtc发送鼠标滚轮消息(conn netclient.NetClient, deviceIds []uint64, upOrDown int, x int, y int) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncScroll, atomic.AddUint32(&c.MsgSeq, 1))
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
func (c *Core) MiddleRtc批量发送设备下载安装任务(deviceIds []uint64, url string, name string, install bool, md5 string, receive bool) {
	//{url:"xxx",md5:"",install:false,name:"filename",receive:true}
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		var sendDeviceIds []uint64
		if ok {
			for _, deviceId := range deviceIds {
				if GetMiddleIdFromDeviceId(deviceId) == mid {
					sendDeviceIds = append(sendDeviceIds, deviceId)
					if len(sendDeviceIds) == 5 {
						c.MiddleRtc发送下载安装任务(MidRtc.Conn, sendDeviceIds, url, name, install, md5, receive)
						sendDeviceIds = sendDeviceIds[:0]
						time.Sleep(time.Millisecond * 50)
					}
				}
			}
			if len(sendDeviceIds) > 0 {
				c.MiddleRtc发送下载安装任务(MidRtc.Conn, sendDeviceIds, url, name, install, md5, receive)
				sendDeviceIds = sendDeviceIds[:0]
			}
		}
	}

}
func (c *Core) MiddleRtc发送下载安装任务(conn netclient.NetClient, deviceIds []uint64, url string, name string, install bool, md5 string, receive bool) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncFileDownload, atomic.AddUint32(&c.MsgSeq, 1))
	downmsg := DownFile{
		Url:     url,
		Md5:     md5,
		Install: install,
		Name:    name,
		Receive: receive,
	}

	if err := msg.Marshal(&downmsg); err != nil {
		logs.Error("公共rtc序列化图片请求消息失败,%s", err.Error())
		return
	} else {
		//logs.Info("序列化图片请求消息成功,msg=%v", imgReq)
	}

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
		} else {
			logs.Info("发送下载安装任务成功,msg=%v", downmsg)
		}
	}
}
func (c *Core) MiddleRtc批量获取设备app列表(deviceIds []uint64) {
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		var sendDeviceIds []uint64
		if ok {
			for _, deviceId := range deviceIds {
				if GetMiddleIdFromDeviceId(deviceId) == mid {
					sendDeviceIds = append(sendDeviceIds, deviceId)
					if len(sendDeviceIds) == 5 {
						c.MiddleRtc发送获取app列表(MidRtc.Conn, sendDeviceIds)
						sendDeviceIds = sendDeviceIds[:0]
						time.Sleep(time.Millisecond * 50)
					}
				}
			}
			if len(sendDeviceIds) > 0 {
				c.MiddleRtc发送获取app列表(MidRtc.Conn, sendDeviceIds)
				sendDeviceIds = sendDeviceIds[:0]
			}
		}
	}
}
func (c *Core) MiddleRtc发送获取app列表(conn netclient.NetClient, deviceIds []uint64) {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncGetAppList, atomic.AddUint32(&c.MsgSeq, 1))
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
		} else {
			logs.Info("MiddleRtc发送获取app列表,msg=%v")
		}
	}

}
func (c *Core) MiddleRtc检测延迟() {
	msg := public.NewMessage(bufferPool.TypeMsgpack, public.FuncTestDelay, atomic.AddUint32(&c.MsgSeq, 1))
	test := DelayRequest{time.Now().UnixMicro()}
	if err := msg.Marshal(&test); err != nil {
		logs.Error("序列化message的data字段出错了,%s", err.Error())
	}

	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		if ok {
			if MidRtc != nil {
				if MidRtc.Conn != nil {
					if err := MidRtc.Conn.SendMsgpack(msg, 0); err != nil {
						logs.Error("发送消息出错了,%s", err.Error())
					}
				}

			}
		}
	}

}
func (c *Core) MiddleRtc批量循环检测延迟() {
	for c.IsClose == false {
		c.MiddleRtc检测延迟()
		time.Sleep(2 * time.Second)
	}
}
func (c *Core) MiddleRtc同步触屏操作(deviceIds []uint64, touchType int, x int, y int, offset int, pressure int, id int) {
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	var phone *DeviceInfo
	var dids []uint64
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		//		logs.Info("同步触屏操作,mid=%d", mid)
		if ok {
			for _, deviceId := range deviceIds {
				phone, ok = c.DidGetPhoneInfo(deviceId)
				if ok {
					if phone.ReqDevice.ProxyId == mid {
						dids = append(dids, deviceId)
						if len(dids) == 30 {
							c.MiddleRtc触屏操作(MidRtc.Conn, dids, touchType, x, y, offset, pressure, id)
							dids = dids[:0]
							time.Sleep(time.Millisecond * 1)
						}

					}
				}
			}
			if len(dids) > 0 {
				c.MiddleRtc触屏操作(MidRtc.Conn, dids, touchType, x, y, offset, pressure, id)
				dids = dids[:0]
			}
		}
	}
}
func (c *Core) MiddleRtc同步鼠标滚轮操作(deviceIds []uint64, 滚动方向 int, x int, y int) {
	mids := c.GetAllMid()
	var dids []uint64
	var ok bool
	var phone *DeviceInfo
	var MidRtc *MiddleRtc
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		if ok {
			for _, deviceId := range deviceIds {
				phone, ok = c.DidGetPhoneInfo(deviceId)
				if ok {
					if phone.ReqDevice.ProxyId == mid {
						dids = append(dids, deviceId)
						if len(dids) == 20 {
							c.MiddleRtc发送鼠标滚轮消息(MidRtc.Conn, dids, 滚动方向, x, y)
							dids = dids[:0]
							time.Sleep(time.Millisecond * 5)
						}
					}
				}
			}
			if len(dids) > 0 {
				c.MiddleRtc发送鼠标滚轮消息(MidRtc.Conn, dids, 滚动方向, x, y)
				dids = dids[:0]
			}

		}
	}
}
func (c *Core) MiddleRtc同步操作键盘消息(deviceIds []uint64, keycode int, action int) int {
	mids := c.GetAllMid()
	var dids []uint64
	var ok bool
	var phone *DeviceInfo
	var MidRtc *MiddleRtc
	for _, mid := range mids {

		MidRtc, ok = c.MidGetConn(mid)
		if ok {
			for _, deviceId := range deviceIds {
				phone, ok = c.DidGetPhoneInfo(deviceId)
				if ok {
					if phone.ReqDevice.ProxyId == mid {
						dids = append(dids, deviceId)
						if len(dids) == 20 {
							c.MiddleRtc发送键盘消息(MidRtc.Conn, dids, keycode, action)
							dids = dids[:0]
							time.Sleep(time.Millisecond * 5)
						}
					}
				}
			}
			if len(dids) > 0 {
				c.MiddleRtc发送键盘消息(MidRtc.Conn, dids, keycode, action)
				dids = dids[:0]
			}
		}
	}
	return 0
}
func (c *Core) MiddleRtc批量心跳() {
	var mids []uint64
	for {
		mids = c.GetAllMid()
		var MidRtc *MiddleRtc
		var ok bool
		for _, mid := range mids {
			MidRtc, ok = c.MidGetConn(mid)
			if ok {
				if MidRtc.Conn != nil {
					err := MidRtc.Conn.SendPing()
					if err != nil {
						atomic.AddInt32(&MidRtc.Heartbeat, -1)
						//logs.Error("中间件[%d]链接心跳发送失败,%s,当前:%d", mid, err.Error(), atomic.LoadInt32(&MidRtc.Heartbeat))
					} else {
						atomic.AddInt32(&MidRtc.Heartbeat, -1)
						//logs.Info("中间件[%d]链接心跳发送成功,当前:%d", mid, atomic.LoadInt32(&MidRtc.Heartbeat))
					}
					if atomic.LoadInt32(&MidRtc.Heartbeat) <= -3 {
						err = MidRtc.Conn.Close()
						if err != nil {
							logs.Error("[MiddleRtc批量心跳]Conn.Close() err:", err, "mid:", mid)
						}
						atomic.StoreInt32(&MidRtc.Heartbeat, 1)
					}
				}

			}

		}
		time.Sleep(Middle心跳间隔 * time.Second)
	}
}
func (c *Core) Rtc批量检查延迟() {
	var deviceIds []uint64
	var phones []*DeviceInfo
	for S.IsClose == false {
		deviceIds = deviceIds[:0]
		deviceIds, phones = S.GetAllDeviceInfo()
		for n := 0; n < len(phones); n++ {
			if atomic.LoadInt32(&phones[n].WindowISOpened) == 1 {
				if phones[n].RtcConn != nil {
					phones[n].Rtc延迟检测()
				}
			}
		}
		time.Sleep(Rtc延迟检测间隔 * time.Millisecond)
	}
}
func (c *Core) Rtc批量检查心跳() {
	var deviceIds []uint64
	var phones []*DeviceInfo
	for S.IsClose == false {
		deviceIds = deviceIds[:0]
		deviceIds, phones = S.GetAllDeviceInfo()
		for n := 0; n < len(phones); n++ {
			if atomic.LoadInt32(&phones[n].WindowISOpened) == 1 {
				if phones[n].RtcConn != nil {
					err := phones[n].RtcConn.SendPing()
					if err != nil {
						logs.Error("设备[%d]发送心跳失败：%v,%d", phones[n].DeviceId, err, atomic.LoadInt32(&phones[n].Heartbeat))
						atomic.AddInt32(&phones[n].Heartbeat, -1)
					}
					atomic.AddInt32(&phones[n].Heartbeat, -1)
					//					logs.Error("设备[%d]发送心跳成功：%d", phones[n].DeviceId, atomic.LoadInt32(&phones[n].Heartbeat))
					if atomic.LoadInt32(&phones[n].Heartbeat) <= -3 {
						err = phones[n].RtcConn.Close()
						if err != nil {
							logs.Error("[Rtc批量检查心跳]Conn.Close() err:", err, "mid:", phones[n].DeviceId)
						}
						atomic.StoreInt32(&phones[n].Heartbeat, 1)
						logs.Error("设备[%d]发生连续心跳失败8次，重连", phones[n].DeviceId, atomic.LoadInt32(&phones[n].Heartbeat))
					}
				}
			}
		}
		time.Sleep(Rtc心跳间隔 * time.Second)
	}
}
func (c *Core) Core全局判断开启H264() {
	for c.IsClose == false {
		_, phones := c.GetAllDeviceInfo()
		for n := 0; n < len(phones); n++ {
			if atomic.LoadInt32(&phones[n].WindowIsOpen) == 1 && atomic.LoadInt32(&phones[n].H264IsOpen) == 0 && phones[n].RtcConn != nil && atomic.LoadInt32(&phones[n].H264开启判断计次) == 0 && atomic.LoadInt64(&phones[n].Time连接成功时间戳) > 0 && time.Now().UnixMilli()-atomic.LoadInt64(&phones[n].Time连接成功时间戳) > 100 && phones[n].Online&2 > 0 {
				logs.Info("设备[%d]开始发送开启H264命令,连接后时间%d,手机在线状态%d", phones[n].DeviceId, time.Now().UnixMilli()-atomic.LoadInt64(&phones[n].Time连接成功时间戳), phones[n].Online)
				go phones[n].Rtc开启H264串流()
				atomic.AddInt32(&phones[n].H264开启判断计次, 1)
				if atomic.LoadInt32(&phones[n].H264开启判断计次) > 500 {
					atomic.StoreInt32(&phones[n].H264开启判断计次, 0)
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func (c *Core) MiddleRtc锁定输入法(conn netclient.NetClient, deviceIds []uint64, 输入法包名 string) {
	var msg *public.Message

	//public.FuncChangeInput对应值为518
	msg = public.NewMessage(bufferPool.TypeMsgpack, public.FuncChangeInput, atomic.AddUint32(&c.MsgSeq, 1))

	msg.Req = true
	msg.Seq = atomic.AddUint32(&c.MsgSeq, 1)

	type InputChange struct {
		Ime string `json:"imeId" msgpack:"imeId"`
	}

	shellcmdReq := InputChange{
		Ime: 输入法包名,
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
			logs.Error("MiddleRtc发送切换输入法命令失败,%s", err.Error())
			return
		}
	}
}
func (c *Core) MiddleRtc批量锁定输入法(deviceIds []uint64, 输入法包名 string) {
	mids := c.GetAllMid()
	var MidRtc *MiddleRtc
	var ok bool
	for _, mid := range mids {
		MidRtc, ok = c.MidGetConn(mid)
		var sendDeviceIds []uint64
		if ok {
			for _, deviceId := range deviceIds {
				if GetMiddleIdFromDeviceId(deviceId) == mid {
					sendDeviceIds = append(sendDeviceIds, deviceId)
					if len(sendDeviceIds) == 5 {
						c.MiddleRtc锁定输入法(MidRtc.Conn, sendDeviceIds, 输入法包名)
						sendDeviceIds = sendDeviceIds[:0]
						time.Sleep(time.Millisecond * 50)
					}
				}
			}
			if len(sendDeviceIds) > 0 {
				c.MiddleRtc锁定输入法(MidRtc.Conn, sendDeviceIds, 输入法包名)
				sendDeviceIds = sendDeviceIds[:0]
			}
		}
	}
}
