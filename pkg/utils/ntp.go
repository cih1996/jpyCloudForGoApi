//go:build linux

package utils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/kardianos/service"
	"os"
	"path/filepath"
	"strings"
)

const syncdScript = `[Time]
NTP=%s
FallbackNTP=0.debian.pool.ntp.org 1.debian.pool.ntp.org 2.debian.pool.ntp.org 3.debian.pool.ntp.org
RootDistanceMaxSec=5
PollIntervalMinSec=32
PollIntervalMaxSec=2048
ConnectionRetrySec=30
SaveIntervalSec=60
`

const (
	serviceTimeSyncd  = "systemd-timesyncd"
	serviceTimeChrony = "chrony"

	dirTimeSyncd = "/etc/systemd/timesyncd.conf.d"
	dirChrony    = "/etc/chrony/conf.d"
	myConfName   = "ibox-ntp.conf"

	chronyTemple = "server %s iburst prefer minpoll 6 maxpoll 10\n"
)

type Ntp struct {
	sv service.Service
}

func (s *Ntp) Init() (err error) {
	s1, err := service.New(&program{}, &service.Config{Name: serviceTimeChrony})
	if err != nil {
		return err
	}
	status, err := s1.Status()
	if err == nil {
		if status == service.StatusRunning {
			s.sv = s1
			return nil
		}
	}

	s2, err := service.New(nil, &service.Config{Name: serviceTimeSyncd})
	if err != nil {
		return
	}
	status2, err := s2.Status()
	if err == nil {
		if status2 == service.StatusRunning {
			s.sv = s2
			return nil
		}
	}
	return errors.New("未找到支持的时间同步服务")
}
func (s *Ntp) GetConfig() []string {
	switch s.sv.String() {
	case serviceTimeSyncd:
		return s.getTimeSyncd()
	case serviceTimeChrony:
		return s.getChrony()
	default:
		return nil
	}
}
func (s *Ntp) getTimeSyncd() []string {
	path := filepath.Join(dirTimeSyncd, myConfName)
	_, err := os.Stat(path)
	if err != nil {
		return nil
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil
	}
	defer f.Close()

	br := bufio.NewReader(f)
	for {
		line, _, err := br.ReadLine()
		if err != nil {
			return nil
		}
		line = bytes.TrimSpace(line)
		sl := bytes.SplitN(line, []byte("="), 2)
		length := len(sl)
		if length > 0 {
			if string(sl[0]) == "NTP" {
				if length > 1 {
					ntps := strings.TrimSpace(string(sl[1]))
					return strings.Split(ntps, " ")
				}
			}
		}
	}
}
func (s *Ntp) getChrony() (ntps []string) {
	path := filepath.Join(dirChrony, myConfName)
	_, err := os.Stat(path)
	if err != nil {
		return nil
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil
	}
	defer f.Close()

	br := bufio.NewReader(f)
	for {
		line, _, err := br.ReadLine()
		if err != nil {
			return nil
		}
		sl := bytes.Split(line, []byte(" "))
		if len(sl) > 3 {
			if string(sl[0]) == "server" {
				ntps = append(ntps, string(sl[1]))
			}
		}
	}
}
func (s *Ntp) SetConfig(ntps []string) error {
	switch s.sv.String() {
	case serviceTimeSyncd:
		return s.setTimeSyncd(ntps)
	case serviceTimeChrony:
		return s.setChony(ntps)
	default:
		return errors.New("未找到支持的时间同步服务")
	}
}
func (s *Ntp) setTimeSyncd(ntps []string) error {
	path := filepath.Join(dirTimeSyncd, myConfName)
	_, err := os.Stat(dirTimeSyncd)
	if err != nil {
		os.MkdirAll(dirTimeSyncd, 0644)
	}
	if len(ntps) == 0 {
		return os.Remove(path)
	} else {
		str := ""
		for _, s2 := range ntps {
			str = str + " " + s2
		}
		buf := fmt.Sprintf(syncdScript, str)
		return os.WriteFile(path, []byte(buf), 0644)
	}
}
func (s *Ntp) setChony(ntps []string) error {
	path := filepath.Join(dirChrony, myConfName)
	var buf bytes.Buffer
	if len(ntps) == 0 {
		return os.Remove(path)
	} else {
		for _, str := range ntps {
			buf.WriteString(fmt.Sprintf(chronyTemple, str))
		}
		buf.WriteString("makestep 1.0 3\n")
		buf.WriteString("rtcsync\n")
		return os.WriteFile(path, buf.Bytes(), 0644)
	}
}
func (s *Ntp) SyncNow() error {
	return s.sv.Restart()
}

type program struct{}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *program) run() {
	// Do work here
}
func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}
