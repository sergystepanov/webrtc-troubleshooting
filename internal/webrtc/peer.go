package webrtc

import (
	"log"
	"net"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

type (
	Connection struct {
		api    *webrtc.API
		config *webrtc.Configuration
	}
	Config struct {
		DisableDefaultInterceptors bool
		DtlsRole                   int
		IceIpMap                   string
		IceLite                    bool
		IcePortMin                 int
		IcePortMax                 int
		IceServers                 []webrtc.ICEServer
		SinglePort                 int
	}
)

var settings webrtc.SettingEngine

func DefaultConnection(conf Config) (*Connection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	i := &interceptor.Registry{}
	if !conf.DisableDefaultInterceptors {
		if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
			return nil, err
		}
	}

	settingEngine := webrtc.SettingEngine{}
	if conf.DtlsRole > 0 {
		log.Printf("A custom DTLS role [%v]", conf.DtlsRole)
		if err := settingEngine.SetAnsweringDTLSRole(webrtc.DTLSRole(conf.DtlsRole)); err != nil {
			panic(err)
		}
	}
	if conf.IceLite {
		settingEngine.SetLite(conf.IceLite)
	}
	if conf.IcePortMin > 0 && conf.IcePortMax > 0 {
		if err := settingEngine.SetEphemeralUDPPortRange(uint16(conf.IcePortMin), uint16(conf.IcePortMax)); err != nil {
			panic(err)
		}
	} else {
		if conf.SinglePort > 0 {
			udpListener, err := net.ListenUDP("udp", &net.UDPAddr{
				IP:   net.IP{0, 0, 0, 0},
				Port: conf.SinglePort,
			})
			if err != nil {
				panic(err)
			}
			log.Printf("Listening for WebRTC traffic at %s", udpListener.LocalAddr())
			settingEngine.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udpListener))
		}
	}
	if conf.IceIpMap != "" {
		settingEngine.SetNAT1To1IPs([]string{conf.IceIpMap}, webrtc.ICECandidateTypeHost)
	}
	settings = settingEngine

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
		config: &peerConf,
	}
	return &conn, nil
}

func (p *Connection) NewConnection() (*webrtc.PeerConnection, error) {
	return p.api.NewPeerConnection(*p.config)
}
