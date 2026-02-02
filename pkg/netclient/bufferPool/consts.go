package bufferPool

const (
	TypeUnknown uint8 = iota
	TypePing
	TypePong
	TypeTestDelayRequest  //延迟测试请求  |type|4|目标对象|时间戳|  当目标对象不为0时,应当转发给目标对象,否则应当立即回复而不是继续转发
	TypeTestDelayResponse //延迟测试回复  |type|4|目标对象|时间戳|
	TypeBinary            //纯二进制数据
	TypeMsgpack
	TypeJson
	TypeText
	TypeVideo
	TypeAudio
	TypeSpeedTest
	TypeSecretMsgpack //加密的msgpack数据,加密方法由通信双方自行决定.手机侧收到数据解密后为: [data,sign,publicKey],服务端收到的返回值解密后为data
	TypeTerminal      //terminal流
)

// [uint32 length][uint8 type][uint8 headerLength][header bytes][payload]
