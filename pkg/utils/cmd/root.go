package cmd

import (
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"runtime"
)

var Version = "unknown"

type MyCMD struct {
	root *cobra.Command
	run  func(cmd *cobra.Command, args []string)
	cfg  *service.Config
	exit chan struct{}
}

/*
New 内部已内置命令: install, uninstall, start, stop, restart, version.
参数 commands 用于从外部传入自定义命令.

	svcConfig := &service.Config{
		Name:        "MyService",
		DisplayName: "MyService",
		Description: "ServiceDescription",
		Option:      make(service.KeyValue),
	}

	if err := cmd.New(svcConfig, func(cmd *cobra.Command, args []string) {
		// 此处执行用户代码
	}).Execute(); err != nil {
		fmt.Println(err)
	}

参数 svcConfig.Dependencies为空，将使用内置。
外置案例：

	svcConfig.Dependencies = []string{
					"Requires=mariadb.target",
					"After=network.target mariadb.target"}
*/
func New(svcConfig *service.Config, killMode bool, run func(cmd *cobra.Command, args []string), commands ...*cobra.Command) *MyCMD {
	if svcConfig == nil {
		panic("service config error")
	}
	svcConfig.Arguments = append(svcConfig.Arguments, "service")
	if runtime.GOOS != "windows" {
		if len(svcConfig.Dependencies) == 0 {
			svcConfig.Dependencies = []string{
				"Requires=network.target",
				"After=network-online.target syslog.target"}
		}
		if killMode {
			svcConfig.Option["SystemdScript"] = SystemdScriptKillMode
		} else {
			svcConfig.Option["SystemdScript"] = SystemdScript
		}
		svcConfig.Option["SysvScript"] = SysvScript
	}

	c := &MyCMD{
		cfg: svcConfig,
		run: run,
		root: &cobra.Command{
			Use:   svcConfig.Name,
			Short: svcConfig.Description,
			Long:  svcConfig.Description,
			Run: func(cmd *cobra.Command, args []string) {
				// 不带任何参数执行, 打印帮助信息
				_ = cmd.Help()
			},
		},
		exit: make(chan struct{}),
	}
	for i := 0; i < len(serviceCMD); i++ {
		if serviceCMD[i].Run == nil {
			serviceCMD[i].Run = c.serviceControl //为服务类参数挂上控制函数
		}
		c.root.AddCommand(serviceCMD[i])
	}
	// 处理自定义命令
	for i := 0; i < len(commands); i++ {
		c.root.AddCommand(commands[i])
	}
	return c
}

func (s *MyCMD) Execute() error {
	return s.root.Execute()
}
