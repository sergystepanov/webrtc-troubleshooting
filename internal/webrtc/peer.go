package webrtc

import (
	"fmt"
	"net"
	"strings"

	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v3"
)

type (
	Connection struct {
		*webrtc.PeerConnection

		api      *webrtc.API
		config   *webrtc.Configuration
		listener *net.UDPConn
	}
	Config struct {
		DisableDefaultInterceptors bool
		DtlsRole                   int
		IceLite                    bool
		IcePortMin                 int
		IcePortMax                 int
		IceServers                 []webrtc.ICEServer
		Logger                     logging.LoggerFactory
		Nat1to1                    string
		SinglePort                 int
	}
)

var settings webrtc.SettingEngine

func DefaultConnection(conf Config) (*Connection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	log := conf.Logger.NewLogger("conf")

	i := &interceptor.Registry{}
	if !conf.DisableDefaultInterceptors {
		if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
			return nil, err
		}
	} else {
		log.Debugf("Default interceptors have been disabled")
	}

	var udpConn *net.UDPConn

	se := webrtc.SettingEngine{}
	if conf.Logger != nil {
		se = webrtc.SettingEngine{LoggerFactory: conf.Logger}
	}
	if conf.DtlsRole > 0 {
		log.Debugf("A custom DTLS role [%v]", conf.DtlsRole)
		if err := se.SetAnsweringDTLSRole(webrtc.DTLSRole(conf.DtlsRole)); err != nil {
			return nil, err
		}
	}
	if conf.IceLite {
		se.SetLite(conf.IceLite)
	}
	if conf.IcePortMin > 0 && conf.IcePortMax > 0 {
		if err := se.SetEphemeralUDPPortRange(uint16(conf.IcePortMin), uint16(conf.IcePortMax)); err != nil {
			return nil, err
		}
	} else {
		if conf.SinglePort > 0 {
			udpListener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IP{0, 0, 0, 0}, Port: conf.SinglePort})
			if err != nil {
				return nil, err
			}
			udpConn = udpListener
			log.Debugf("Listening for WebRTC traffic at %s", udpListener.LocalAddr())
			se.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udpListener))
		}
	}
	if conf.Nat1to1 != "" {
		if ip, ct, err := parseNatCandidate(conf.Nat1to1); err == nil {
			se.SetNAT1To1IPs(ip, ct)
			log.Debugf("Using 1:1 NAT %s", conf.Nat1to1)
		} else {
			log.Errorf("NAT map error: %v", err)
		}
	}
	settings = se

	peerConf := webrtc.Configuration{ICEServers: []webrtc.ICEServer{}}
	if len(conf.IceServers) > 0 {
		peerConf.ICEServers = conf.IceServers
	}

	conn := Connection{
		api: webrtc.NewAPI(
			webrtc.WithMediaEngine(m),
			webrtc.WithInterceptorRegistry(i),
			webrtc.WithSettingEngine(settings),
		),
		config:   &peerConf,
		listener: udpConn,
	}
	return &conn, nil
}

func parseNatCandidate(v string) (ips []string, candidateType webrtc.ICECandidateType, err error) {
	parts := strings.Split(v, "/")
	if len(parts) < 2 {
		return nil, 0, fmt.Errorf("wrong ICE IP NAT mapping format, %v", parts)
	}
	ips = []string{parts[0]}
	candidateType, err = webrtc.NewICECandidateType(parts[1])
	return
}

func (p *Connection) Connect() error {
	pc, err := p.api.NewPeerConnection(*p.config)
	if err != nil {
		return err
	}
	p.PeerConnection = pc
	return nil
}

func (p *Connection) Close() error {
	var err error
	if p.listener != nil {
		err = p.listener.Close()
	}
	if p.PeerConnection != nil {
		err = p.PeerConnection.Close()
	}
	return err
}
