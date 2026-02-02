package streamRTC

import (
	"encoding/json"
)

type Message struct {
	Fun   string          `json:"Fun" msgpack:"fun" validate:"required"`
	Data  json.RawMessage `json:"Data" msgpack:"data" validate:"required"`
	Code  int             `json:"Code,omitempty" msgpack:"code" validate:"omitempty"`
	Msg   string          `json:"Msg,omitempty" msgpack:"msg" validate:"omitempty"`
	MsgId int32           `json:"MsgId,omitempty" msgpack:"id" validate:"omitempty"`
}

func (s *Message) ToString() string {
	return string(s.Marshal())
}
func (s *Message) Marshal() []byte {
	if buf, err := json.Marshal(s); err != nil {
		msg := Message{Fun: s.Fun, Code: s.Code, Msg: err.Error(), MsgId: s.MsgId}
		return msg.Marshal()
	} else {
		return buf
	}
}
func (s *Message) Unmarshal(v interface{}) error {
	return json.Unmarshal(s.Data, v)
}
func (s *Message) SetData(v interface{}) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.Data = buf
	return nil
}
