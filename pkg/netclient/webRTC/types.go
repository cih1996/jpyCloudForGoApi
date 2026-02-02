package webRTC

import (
	"github.com/ghp3000/logs"
	"github.com/pion/webrtc/v4"
)

type OnDataChannelCallback func(c *Client, channelId uint16, label string, msg *webrtc.DataChannelMessage)
type StandardCallback func(c *Client, channel *DataChannel)
type LoggerFunc func(f interface{}, v ...interface{})

func DefaultLogger(f interface{}, v ...interface{}) {
	logs.Debug(f, v...)
}

type ICEServerInfo struct {
	Username string   `json:"Username"`
	Password string   `json:"Password"`
	Urls     []string `json:"Urls"`
}
type StatsReport struct {
	webrtc.StatsReport
}

func (s *StatsReport) getNominated() (ids []string) {
	for _, stats := range s.StatsReport {
		switch stats.(type) {
		case webrtc.ICECandidatePairStats:
			ps := stats.(webrtc.ICECandidatePairStats)
			if ps.Nominated {
				ids = append(ids, ps.LocalCandidateID, ps.RemoteCandidateID)
			}
		}
	}
	return
}
func (s *StatsReport) getTyp(id ...string) (ts []webrtc.ICECandidateType) {
	for _, stats := range s.StatsReport {
		switch stats.(type) {
		case webrtc.ICECandidateStats:
			ps := stats.(webrtc.ICECandidateStats)
			for _, i := range id {
				if ps.ID == i {
					ts = append(ts, ps.CandidateType)
				}
			}
		}
	}
	return
}

// Mode 1=直连 2=p2p 4=relay
func (s *StatsReport) Mode() ConnectMode {
	ids := s.getNominated()
	if len(ids) == 0 {
		return 0
	}
	ts := s.getTyp(ids...)
	var host int
	for _, typ := range ts {
		if typ == webrtc.ICECandidateTypeHost {
			host++
		} else {
			if typ == webrtc.ICECandidateTypeSrflx || typ == webrtc.ICECandidateTypePrflx {
				return ConnectMode(webrtc.ICECandidateTypeSrflx)
			}
		}
	}
	if host > 1 {
		return ConnectMode(webrtc.ICECandidateTypeHost)
	}
	return ConnectMode(webrtc.ICECandidateTypeRelay)
}

type ConnectMode int

func (m ConnectMode) String() string {
	switch m {
	case 1:
		return "direct"
	case 2:
		return "p2p"
	case 4:
		return "relay"
	default:
		return "unknown"
	}
}
