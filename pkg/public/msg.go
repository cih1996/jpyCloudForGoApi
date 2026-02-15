package public

import (
	"encoding/json"
	"fmt"
	"github.com/ghp3000/netclient/bufferPool"
	"github.com/vmihailenco/msgpack/v5"
)

type Message struct {
	F           uint16             `json:"f" msgpack:"f" validate:"required"`                             //函数名
	Req         bool               `json:"req" msgpack:"req" validate:"omitempty"`                        //request:true=请求,false=返回值
	Seq         uint32             `json:"seq" msgpack:"seq" validate:"required"`                         //包序号
	Code        int32              `json:"code,omitempty" msgpack:"code,omitempty"  validate:"omitempty"` //状态码
	Msg         string             `json:"msg,omitempty" msgpack:"msg,omitempty"  validate:"omitempty"`   //状态文本
	T           int32              `json:"t,omitempty" msgpack:"t,omitempty"  validate:"omitempty"`       //time
	DataJson    json.RawMessage    `json:"data,omitempty" msgpack:"-"  validate:"omitempty"`              //数据
	DataMsgpack msgpack.RawMessage `json:"-" msgpack:"data,omitempty"  validate:"omitempty"`              //数据
	Type        uint8              `json:"-" msgpack:"-" validate:"omitempty"`                            //5=msgpack,6=json
}

func NewMessage(typ uint8, F uint16, seq uint32) *Message {
	return &Message{Type: typ, F: F, Seq: seq, Req: true}
}
func (m *Message) SetCode(code Err) *Message {
	m.Code = int32(code)
	m.Msg = code.Error()
	return m
}
func (m *Message) Unmarshal(v interface{}) error {
	if m.Type == bufferPool.TypeJson {
		return json.Unmarshal(m.DataJson, v)
	} else if m.Type == bufferPool.TypeMsgpack {
		return msgpack.Unmarshal(m.DataMsgpack, v)
	}
	return fmt.Errorf("unknown data type:%d", m.Type)
}
func (m *Message) Marshal(v interface{}) error {
	if m.Type == bufferPool.TypeJson {
		buf, err := json.Marshal(v)
		if err != nil {
			return err
		}
		m.DataJson = buf
	} else {
		buf, err := msgpack.Marshal(v)
		if err != nil {
			return err
		}
		m.DataMsgpack = buf
	}
	return nil
}
func (m *Message) Error() error {
	if m.Code == int32(Success) {
		return nil
	}
	return Err(m.Code)
}
func (m *Message) Bytes() ([]byte, error) {
	if m.Type == bufferPool.TypeJson {
		return json.Marshal(m)
	} else if m.Type == bufferPool.TypeMsgpack {
		return msgpack.Marshal(m)
	}
	return nil, fmt.Errorf("unknown data type:%d", m.Type)
}

// ToJsonString 将消息数据转换为 JSON 字符串
func (m *Message) ToJsonString() string {
	if m.F == 12 {
		var jsonMsg map[uint8]interface{}
		if err := m.Unmarshal(&jsonMsg); err != nil {
			return ""
		}
		temp := make(map[string]interface{})
		for k, v := range jsonMsg {
			temp[fmt.Sprintf("%d", k)] = v
		}
		jsonData, _ := json.Marshal(temp)
		return string(jsonData)
	}
	var a interface{}
	if err := m.Unmarshal(&a); err != nil {
		return ""
	}
	jsonData, err := json.Marshal(a)
	if err != nil {
		return ""
	}
	return string(jsonData)
}
