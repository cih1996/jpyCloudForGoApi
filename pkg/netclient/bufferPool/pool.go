package bufferPool

import (
	"sync"
	"sync/atomic"
)

const defaultBufferCap = 64 * 1024

var Buffer = New()

type Pool struct {
	pool sync.Pool
}

func New() *Pool {
	return &Pool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Packet{
					refCounter: atomic.Int32{},
					Length:     0,
					Buff:       make([]byte, 0, defaultBufferCap),
				}
			},
		},
	}
}
func (p *Pool) Get() *Packet {
	return p.pool.Get().(*Packet).Ref()
}
func (p *Pool) NewPacket(v []byte) *Packet {
	packet := p.Get()
	packet.InitWithBytes(v)
	return packet
}
func (p *Pool) put(x *Packet) {
	p.pool.Put(x)
}
func Get() *Packet {
	return Buffer.Get()
}
