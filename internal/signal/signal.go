package signal

import (
	"errors"
	"fmt"
	webrtc2 "github.com/sergystepanov/webrtc-troubleshooting/v2/internal/webrtc"
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sergystepanov/webrtc-troubleshooting/v2/pkg/api"
	"golang.org/x/net/websocket"
)

func Signalling() websocket.Handler {
	send, receive := websocket.JSON.Send, websocket.JSON.Receive
	status := func() string { return fmt.Sprintf("%08b", rand.Intn(256)) }

	return func(conn *websocket.Conn) {
		messages, done := make(chan api.Log, 100), make(chan struct{})

		q := conn.Request().URL.Query()

		flip := q.Get("flip_offer_side") == "true"

		go func() {
			for {
				select {
				case <-done:
					return
				case m := <-messages:
					if err := send(conn, api.NewLog(m)); err != nil {
						log.Printf("log err: %v", err)
						return
					}
				}
			}
		}()

		_log := func(tag string, format string, v ...any) string {
			m := fmt.Sprintf(format, v...)
			line := fmt.Sprintf("%s %s", tag, m)
			log.Printf(line)
			messages <- api.Log{Tag: tag, Text: m}
			return line
		}

		s := webrtc.SettingEngine{
			LoggerFactory: webrtc2.CustomLoggerFactory{
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
				if err := send(conn, api.NewIce(*c)); err != nil {
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
			err := peer.Close()
			if err != nil {
				log.Printf("close err: %v", err)
				return
			}
		}()

		for {
			var m api.Message
			err := receive(conn, &m)
			if errors.Is(err, io.EOF) {
				if err := conn.WriteClose(1000); err != nil {
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
				if err = send(conn, api.NewAnswer(answer)); err != nil {
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
				if err = send(conn, api.NewOffer(offer)); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case api.WebrtcClose:
				_log("sig", "!close")
			default:
				_log("sys", "err: unknown message [%v]", m.T)
				return
			}
		}
	}
}
