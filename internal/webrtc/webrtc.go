package webrtc

import (
	"errors"
	"strconv"

	"github.com/pion/logging"
	"github.com/pion/webrtc/v4"
)

type (
	DataChannel struct {
		ch *webrtc.DataChannel
	}
	Peer struct {
		conn *Connection
	}
	State interface {
		~int | ~int32 | ~uint32
		String() string
	}
	ICECandidate        = webrtc.ICECandidate
	ICEConnectionState  = webrtc.ICEConnectionState
	ICEGatheringState   = webrtc.ICEGatheringState
	PeerConnectionState = webrtc.PeerConnectionState
	SignalingState      = webrtc.SignalingState
)

func (dc *DataChannel) OnOpen(fn func()) { dc.ch.OnOpen(fn) }
func (dc *DataChannel) SendText(text string) error {
	if dc.ch.ReadyState() != webrtc.DataChannelStateOpen {
		return nil
	}
	return dc.ch.SendText(text)
}

func NewPeerConnection(iceServers []string, disableInterceptors bool, port, nat1to1 string, noMDNS bool, logger logging.LoggerFactory) (*Peer, error) {
	conf := Config{
		DisableDefaultInterceptors: disableInterceptors,
		DisableMDNS:                noMDNS,
		Nat1to1:                    nat1to1,
		Logger:                     logger,
	}
	if len(iceServers) > 0 {
		var ices []webrtc.ICEServer
		for _, s := range iceServers {
			if s == "" {
				continue
			}
			ices = append(ices, webrtc.ICEServer{URLs: []string{s}})
		}
		conf.IceServers = ices
	}
	if port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			conf.SinglePort = p
		}
	}
	conn, err := DefaultConnection(conf)
	if err != nil {
		return nil, err
	}
	if err = conn.Connect(); err != nil {
		return nil, err
	}
	return &Peer{conn}, nil
}

func (p *Peer) CreateDataChannel(name string) (*DataChannel, error) {
	ch, err := p.conn.CreateDataChannel(name, nil)
	if err != nil {
		return nil, err
	}
	return &DataChannel{ch}, nil
}

func (p *Peer) OnIceCandidate(fn func(c *ICECandidate)) {
	p.conn.OnICECandidate(fn)
}
func (p *Peer) OnIceConnectionStateChange(fn func(state ICEConnectionState)) {
	p.conn.OnICEConnectionStateChange(fn)
}
func (p *Peer) OnConnectionStateChange(fn func(state PeerConnectionState)) {
	p.conn.OnConnectionStateChange(fn)
}
func (p *Peer) OnIceGatheringStateChange(fn func(state ICEGatheringState)) {
	p.conn.OnICEGatheringStateChange(fn)
}
func (p *Peer) OnSignalingStateChange(fn func(state SignalingState)) {
	p.conn.OnSignalingStateChange(fn)
}
func (p *Peer) OnDataChannel(fn func(d *DataChannel)) {
	p.conn.OnDataChannel(func(channel *webrtc.DataChannel) {
		fn(&DataChannel{channel})
	})
}

func (p *Peer) CreateAnswer() (*webrtc.SessionDescription, error) {
	answer, err := p.conn.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}
	if err := p.conn.SetLocalDescription(answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

func (p *Peer) CreateOffer() (*webrtc.SessionDescription, error) {
	offer, err := p.conn.CreateOffer(nil)
	if err != nil {
		return nil, err
	}
	if err := p.conn.SetLocalDescription(offer); err != nil {
		return nil, err
	}
	return &offer, nil
}

func (p *Peer) AddIceCandidate(candidate webrtc.ICECandidateInit) error {
	if candidate.Candidate == "" {
		return nil
	}
	return p.conn.AddICECandidate(candidate)
}

func (p *Peer) Close() error {
	return p.conn.Close()
}

func (p *Peer) SetRemoteSDP(session webrtc.SessionDescription) error {
	if session.SDP == "" {
		return errors.New("empty SDP")
	}
	return p.conn.SetRemoteDescription(session)
}
