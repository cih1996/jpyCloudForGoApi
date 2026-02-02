package public

type Err int32

const (
	Success        Err = iota
	NotConnect         //指定的连接未连接
	NotOnline          //指定的设备不在线
	FormatError        //数据格式错误,解析错误
	Timeout            //执行任务超时
	NotHere            //不允许在这里执行此功能,全局订阅连接上开启h264将返回此错误.
	Unauthorized       //未获取到授权信息
	CertificateErr     //证书错误.不是自己发行的/用途不正确
	InternalError      //内部错误,一般是程序bug性质
	DecryptErr         //加密数据解密失败
	UnsupportedFunc
)

func (e Err) Error() string {
	switch e {
	case Success:
		return "success"
	case NotConnect:
		return "not connect"
	case NotOnline:
		return "not online"
	case FormatError:
		return "data format error"
	case Timeout:
		return "task timeout"
	case NotHere:
		return "not allowed here"
	case Unauthorized:
		return "unauthorized"
	case CertificateErr:
		return "certificate error"
	case InternalError:
		return "internal error"
	case DecryptErr:
		return "decrypt error"
	case UnsupportedFunc:
		return "unsupported function"
	default:
		return "unparser error"
	}
}
