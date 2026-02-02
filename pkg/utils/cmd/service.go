package cmd

import (
	"fmt"
	"github.com/ghp3000/logs"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"runtime/debug"
)

var serviceCMD = []*cobra.Command{
	{
		Use:   "install",
		Short: "Install the service",
	},
	{
		Use:   "uninstall",
		Short: "uninstall the service",
	},
	{
		Use:   "start",
		Short: "start the service",
	},
	{
		Use:   "stop",
		Short: "stop the service",
	},
	{
		Use:   "restart",
		Short: "restart the service",
	},
	{
		Use:   "service",
		Short: "run program as service",
	},
	{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "show version",
		Run: func(cmd *cobra.Command, args []string) {
			print(Version)
			os.Exit(0)
		},
	},
}

func (s *MyCMD) serviceControl(cmd *cobra.Command, args []string) {
	switch cmd.Use {
	case "install":
		if err := s.install(); err != nil {
			fmt.Println("install fail:", err.Error())
		} else {
			fmt.Println("install success")
		}
	case "uninstall":
		if err := s.uninstall(); err != nil {
			fmt.Println("uninstall fail:", err.Error())
		} else {
			fmt.Println("uninstall success")
		}
	case "start", "stop", "restart":
		srv, err := service.New(s, s.cfg)
		if service.Platform() == "unix-systemv" {
			c := exec.Command("/etc/init.d/"+s.cfg.Name, os.Args[1])
			err = c.Run()
			if err != nil {
				fmt.Println(err.Error())
			}
			return
		}
		err = service.Control(srv, cmd.Use)
		if err != nil {
			fmt.Printf(err.Error())
		}
	case "service":
		srv, err := service.New(s, s.cfg)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		if err = srv.Run(); err != nil {
			fmt.Println(err.Error())
		}
	default:
		fmt.Println("unknown arg:", cmd.Use)
	}
}

func (s *MyCMD) install() error {
	srv, err := service.New(s, s.cfg)
	_ = service.Control(srv, "stop")
	_ = service.Control(srv, "uninstall")

	binPath, _ := os.Executable()
	s.cfg.Executable = binPath
	srv, err = service.New(s, s.cfg)
	if err != nil {
		return err
	}
	err = service.Control(srv, "install")
	if err != nil {
		return fmt.Errorf("Valid actions: %q\n%s", service.ControlAction, err.Error())
	}
	if service.Platform() == "unix-systemv" {
		confPath := "/etc/init.d/" + s.cfg.Name
		os.Symlink(confPath, "/etc/rc.d/S90"+s.cfg.Name)
		os.Symlink(confPath, "/etc/rc.d/K02"+s.cfg.Name)
	}
	return nil
}
func (s *MyCMD) uninstall() error {
	srv, err := service.New(s, s.cfg)
	if err != nil {
		return err
	}
	_ = service.Control(srv, "stop")
	err = service.Control(srv, "uninstall")
	if err != nil {
		return err
	}
	if service.Platform() == "unix-systemv" {
		os.Remove("/etc/rc.d/S90" + s.cfg.Name)
		os.Remove("/etc/rc.d/K02" + s.cfg.Name)
	}
	return nil
}

func (s *MyCMD) Start(srv service.Service) error {
	_, _ = srv.Status()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logs.Fatal("%s panic:", s.root.Use, err)
				logs.Fatal(string(debug.Stack()))
			}
		}()
		go s.run(s.root, nil)
		select {
		case <-s.exit:
			logs.Warn("now stop...")
		}
	}()
	return nil
}
func (s *MyCMD) Stop(srv service.Service) error {
	_, _ = srv.Status()
	close(s.exit)
	if service.Interactive() {
		os.Exit(0)
	}
	return nil
}
