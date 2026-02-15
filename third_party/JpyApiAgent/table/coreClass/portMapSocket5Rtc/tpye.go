package portMapSocket5Rtc

import (
	"port-mapping-demo/third_party/JpyApiAgent/table/coreClass/publicStruct"
	"encoding/binary"
	"github.com/ghp3000/logs"
	"github.com/ghp3000/netclient/NetClient"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/ghp3000/public"
	"sync"
	"time"
)

var (
	serverMap sync.Map
)

type TypeInfo struct {
	publicStruct.PortMapTypeInfo
}

func (s *TypeInfo) SyncCall(conn NetClient.ConnWithDeadline, seat uint8, msg *public.Message, timeout time.Duration) *public.Message {
	if conn == nil {
		msg.SetCode(public.NotOnline)
		return msg
	}
	pkt := bufferPool.Get()
	defer pkt.Put()
	if err := pkt.Write(bufferPool.TypeMsgpack, 0, msg); err != nil {
		msg.SetCode(public.InternalError)
		return msg
	}
	if err := binary.Write(conn, binary.LittleEndian, pkt.Length); err != nil {
		msg.SetCode(public.NotConnect)
		return msg
	}
	if _, err := conn.Write(pkt.Bytes()); err != nil {
		msg.SetCode(public.NotOnline)
	}
	err := conn.SetReadDeadline(time.Now().Add(time.Second * 5))
	if err != nil {
		logs.Info("SetReadDeadline failed: %v", err)
	}
	if err := pkt.ReadFromReader(conn); err != nil {
		msg.SetCode(public.NotConnect)
		return msg
	}
	if err := pkt.Unmarshal(msg); err != nil {
		msg.SetCode(public.InternalError)
		return msg
	}
	return msg
}

func save(deviceId uint64, portMap *TypeInfo) {
	serverMap.Store(deviceId, portMap)
}
func get(deviceId uint64) *TypeInfo {
	portMap, ok := serverMap.Load(deviceId)
	if !ok {
		return nil
	}
	return portMap.(*TypeInfo)
}
func del(deviceId uint64) {
	serverMap.Delete(deviceId)
}
func getAll() []*TypeInfo {
	portMapList := make([]*TypeInfo, 0)
	serverMap.Range(func(key, value any) bool {
		portMapList = append(portMapList, value.(*TypeInfo))
		return true
	})
	return portMapList
}
