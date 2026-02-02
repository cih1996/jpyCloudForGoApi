package public

/*
原则上f 类型应当为uint16,高8位表达模块编号,低8位是功能编号.每个功能模块最多支持255个功能.
取模块编号 f>>8, 比如 1>>8 ==0, 256>>8 ==1, 512>>8 ==2
这样设计好处是让手机侧可以通过模块编号决策转发消息给哪个插件.比如插件A 连接到主进程即宣告自己的模块id为2,这样主进程在遇到f=512的时候,即可直接转发给模块A.
0~255,模块编号0, 中间件和集控平台用
256~511,模块编号1,手机端核心进程用
>512 其他插件或组件用


升级包: zip压缩并用鉴权服务器的根证书签名,压缩包内文件结构[config.json,可执行文件,必要的配置文件]
config.json文件需要到授权平台去做签名,签名后的得到的实际文件是 文件字节+64字节的签名
config.json内容: 插件id,插件名称,插件版本,插件描述,执行文件名,执行文件md5,启动参数
zip打包好之后,也上传到授权平台去签名.
*/

const (
	FuncTestDelay       = 1  //延迟测试
	FuncAddPermit       = 2  //集控平台向中间件发送添加用户权限
	FuncDelPermit       = 3  //集控平台向中间件发送移除用户权限
	FuncDevice          = 4  //一台设备的信息设备上线会主动推送,中间件被动接收
	FuncDevices         = 5  //设备列表(本地缓存一份列表,只有当设备信息有变化或集控平台主动请求,才发送)
	FuncOnlineList      = 6  //每次连接成功向集控平台推送设备的在线情况列表.(仅在线或离线的状态,不包含其他信息,省流)
	FuncTunnel          = 7  //集控平台向中间件发送打洞请求(前端的屏幕墙连接,串流连接,都从这里开始)
	FuncLogin           = 8  //
	FuncTerm            = 9  //开启或关闭shell串流(websocket->中间件->手机) 参数:public.TermAction
	FuncVersionSequence = 10 //{"file":1,"device2version":1}升级包总版本号和盘位vs插件版本发行序号.集控平台管理所有插件的版本,有个总版本号,每次修改了插件版本,此版本号可加1.中间件获取后和本地缓存版本号比较,决定是否要下载版本列表
	FuncPlugins         = 11 //下载版本列表{"version":111,"list":[]Version}
	FuncDevice2Version  = 12 //手机和版本的关联关系["deviceId":{"main":1,"plugin":[]}]
	FuncMgtDevices      = 13 //集中管理平台专用,带缓存差异推送设备信息变化
	FuncBatchCommand    = 14 //执行单句shell命令,比如  ls -l. 发送string返回string
	FuncExit            = 15 //命令进程强制退出(触发进程重启)
	FuncPortMap         = 16 //开始映射的信令
	FuncPortTunnel      = 17 //集控平台向中间件发送打洞请求(端口映射专用)

	FuncFileServer = 100 //设置httpCache地址
	//FuncBroadcastInfo 广播类消息,用途1:中间件将采集到的设备信息广播给所有用户.用途2:udp广播扫描取设备基本信息(中间件广播,手机armLinux回应)
	FuncBroadcastInfo          = 101
	FuncPreSetting             = 102 //udp发送预分配的手机设置(dhcp发送,手机armLinux接收)
	FuncGetSettingByHid        = 103 //armLinux在hid硬件标志存在的情况下,发送sn,中间件返回签名后的Setting
	FuncGetSettingByPreSetting = 104 //armLinux发送PreSetting,中间件返回签名后的Setting
	FuncDeviceVersion          = 105 //手机内程序版本: {"name":"main","id":1,"version":"","url":""}

	FuncScreenChange  = 250 //设备屏幕旋转广播
	FuncStartVideo    = 251 //开启设备的视频编码,群控连接禁止调用
	FuncStopVideo     = 252 //停止设备的视频编码,群控连接禁止调用
	FuncStartAudio    = 253 //开启设备的音频编码,群控连接禁止调用
	FuncStopAudio     = 254 //停止设备的音频编码,群控连接禁止调用
	FuncTouch         = 258 //触摸 [{type: 0, x: 100, y: 200, id: 1,offset:10,pressure:1}] type：0按下1抬起2移动
	FuncScroll        = 259 //滚动 {upOrDown: -1, x: 100, y: 100}
	FuncKey           = 281 //发送按键.{action: 3, keyCode: 3}	action：0按下1抬起3按下并抬起，4ctrol+组合键 keyCode：键码
	FuncCMD           = 288 //{shell: "ls -l /data/local/tmp"}
	FuncCMDWithResult = 289 //{shell: "ls -l /data/local/tmp"}
	FuncGetAppList    = 290 //获取设备app列表
	FuncRunApp        = 291 // 运行app
	FuncWakeUPAways   = 298 //屏幕常亮
	FuncImg           = 299 //{x:0,y:0,width: w, height: h, qua: 90,scale:1080}

	FuncAuth         = 255 //设备鉴权
	FuncFileDownload = 293
	FuncChangeInput  = 518 //切换输入源
	FuncInputText    = 769 //输入文本
	FuncGetText      = 770 //获取远程手机剪辑版内容
	FuncChangSwitch  = 515
	FuncSetRootApp   = 516 // 设置root app
	FuncUnSetRootApp = 517 // 取消设置root app
)
