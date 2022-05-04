package signal

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/pion/webrtc/v3"
	pion "github.com/sergystepanov/webrtc-troubleshooting/v2/internal/webrtc"
	"github.com/sergystepanov/webrtc-troubleshooting/v2/pkg/api"
	"golang.org/x/net/websocket"
)

type socket struct {
	*websocket.Conn
	closed bool
}

func (s *socket) close()                      { s.closed = true }
func (s *socket) receive(m interface{}) error { return websocket.JSON.Receive(s.Conn, m) }
func (s *socket) send(m interface{}) error {
	if s.closed {
		return nil
	}
	return websocket.JSON.Send(s.Conn, m)
}

func Signalling() websocket.Handler {
	status := func() string { return fmt.Sprintf("%08b", rand.Intn(256)) }

	return func(wc *websocket.Conn) {
		signal := socket{wc, false}
		done := make(chan struct{})

		q := signal.Request().URL.Query()

		flip := q.Get("flip_offer_side") == "true"

		mes := func(m api.Log) {
			if err := signal.send(api.NewLog(m)); err != nil {
				log.Printf("log err: %v", err)
			}
		}

		_log := func(tag string, format string, v ...any) string {
			m := fmt.Sprintf(format, v...)
			line := fmt.Sprintf("%s %s", tag, m)
			log.Printf(line)
			mes(api.Log{Tag: tag, Text: m})
			return line
		}

		s := webrtc.SettingEngine{
			LoggerFactory: pion.CustomLoggerFactory{
				Logg: _log,
			},
		}
		apii := webrtc.NewAPI(webrtc.WithSettingEngine(s))

		peer, err := apii.NewPeerConnection(webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
		})
		if err != nil {
			panic(err)
		}

		if flip {
			dc, err := peer.CreateDataChannel("data", nil)
			if err != nil {
				panic(err)
			}

			dc.OnOpen(func() {
				send := func(message string) {
					if dc.ReadyState() == webrtc.DataChannelStateOpen {
						if err = dc.SendText(message); err != nil {
							panic(err)
						}
					}
				}
				send(status())
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						_log("sys", "STOP TICKER!")
						return
					case _ = <-ticker.C:
						send(status())
					}
				}
			})
		}

		peer.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c != nil {
				if err := signal.send(api.NewIce(*c)); err != nil {
					panic(err)
				}
			}
		})

		peer.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) { _log("ice", "→ %s", state) })
		peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) { _log("rtc", "→ %s", state) })
		peer.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) { _log("ice", "→ %s", state) })
		peer.OnSignalingStateChange(func(state webrtc.SignalingState) { _log("sig", "→ %s", state) })

		peer.OnDataChannel(func(d *webrtc.DataChannel) {
			send := func(message string) {
				if d.ReadyState() == webrtc.DataChannelStateOpen {
					if err = d.SendText(message); err != nil {
						panic(err)
					}
				}
			}
			d.OnOpen(func() {
				send(status())
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						_log("sys", "STOP TICKER!")
						return
					case _ = <-ticker.C:
						send(status())
					}
				}
			})
		})

		defer func() {
			// !to wait for message drain

			done <- struct{}{}
			signal.close()
		}()

		for {
			var m api.Message
			err := signal.receive(&m)
			if errors.Is(err, io.EOF) {
				if err := signal.WriteClose(1000); err != nil {
					log.Printf("error: failed signal close, %v", err)
				}
				log.Printf("Signal has been closed!")
				return
			} else if err != nil {
				_log("sys", "err: %v", err)
				continue
			}

			switch m.T {
			case api.WebrtcAnswer:
				if answer, err := api.NewSessionDescription(m.Payload); err == nil && answer.SDP != "" {
					if err = peer.SetRemoteDescription(answer.SessionDescription); err != nil {
						_log("rtc", "err: %v", err)
						return
					}
				}
			case api.WebrtcOffer:
				if offer, err := api.NewSessionDescription(m.Payload); err == nil && offer.SDP != "" {
					if err = peer.SetRemoteDescription(offer.SessionDescription); err != nil {
						_log("rtc", "err: %v", err)
						return
					}
				}
				answer, err := peer.CreateAnswer(nil)
				if err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if err = peer.SetLocalDescription(answer); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if err = signal.send(api.NewAnswer(answer)); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case api.WebrtcIce:
				if candidate, err := api.NewIceCandidateInit(m.Payload); err == nil && candidate.Candidate != "" {
					if err = peer.AddICECandidate(candidate); err != nil {
						_log("ice", "err: %v", err)
						return
					}
				}
			case api.WebrtcWaitingOffer:
				offer, err := peer.CreateOffer(nil)
				if err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if err = peer.SetLocalDescription(offer); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if err = signal.send(api.NewOffer(offer)); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case api.WebrtcClose:
				_log("sig", "!close")
				err := peer.Close()
				if err != nil {
					log.Printf("close err: %v", err)
				}
				if err = signal.send(api.NewClose()); err != nil {
					_log("sig", "err: %v", err)
					return
				}
			default:
				_log("sys", "err: unknown message [%v]", m.T)
				return
			}
		}
	}
}
