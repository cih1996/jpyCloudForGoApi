package portMapSocket5Rtc

const (
	ModePortMaps         = 0 //连接复用的端口映射
	ModeSocket5Server    = 2 //使用手机的socks5代理服务模式
	CodePortMaps开始Rtc连接  = 1
	MsgPortMaps开始Rtc连接   = "端口映射开始Rtc连接"
	CodePortMapsRtc连接建立  = 2
	MsgPortMapsRtc连接建立   = "端口映射Rtc连接建立"
	CodePortMapsRtc连接成功  = 3
	MsgPortMapsRtc连接成功   = "端口映射Rtc连接成功"
	CodePortMapsRtc连接失败  = 4
	MsgPortMapsRtc连接失败   = "端口映射Rtc连接失败"
	CodePortMaps本地端口开启开始 = 5
	MsgPortMaps本地端口开启开始  = "端口映射本地映射开始"
	CodePortMaps本地端口开启失败 = 6
	MsgPortMaps本地端口开启失败  = "端口映射本地端口开启失败"
	CodePortMaps本地端口开启成功 = 7
	MsgPortMaps本地端口开启成功  = "端口映射本地端口开启成功"
	CodePortMaps成功       = 100
	MsgPortMaps成功        = "端口映射成功"
	CodePortMaps断开连接     = 101
	MsgPortMaps断开连接      = "端口映射断开连接"

	CodeSocket5Server开始Rtc连接  = 11
	MsgSocket5Server开始Rtc连接   = "Socket5Server开始Rtc连接"
	CodeSocket5ServerRtc连接建立  = 12
	MsgSocket5ServerRtc连接建立   = "Socket5ServerRtc连接建立"
	CodeSocket5ServerRtc连接成功  = 13
	MsgSocket5ServerRtc连接成功   = "Socket5ServerRtc连接成功"
	CodeSocket5ServerRtc连接失败  = 14
	MsgSocket5ServerRtc连接失败   = "Socket5ServerRtc连接失败"
	CodeSocket5Server本地端口开启开始 = 15
	MsgSocket5Server本地端口开启开始  = "Socket5Server本地端口开启开始"
	CodeSocket5Server本地端口开启失败 = 16
	MsgSocket5Server本地端口开启失败  = "Socket5Server本地端口开启失败"
	CodeSocket5Server本地端口开启成功 = 17
	MsgSocket5Server本地端口开启成功  = "Socket5Server本地端口开启成功"
	CodeSocket5Server成功       = 200
	MsgSocket5Server成功        = "Socket5Server成功"
	CodeSocket5Server断开连接     = 201
	MsgSocket5Server断开连接      = "Socket5Server断开连接"
)
