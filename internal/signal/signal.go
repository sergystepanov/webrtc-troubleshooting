package signal

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sergystepanov/webrtc-troubleshooting/v2/internal/stun"
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

func remoteLogger(s *socket) func(tag string, format string, v ...any) string {
	return func(tag string, format string, v ...any) string {
		m := fmt.Sprintf(format, v...)
		line := fmt.Sprintf("%s %s", tag, m)
		log.Printf(line)
		if err := s.send(api.NewLog(api.Log{Tag: tag, Text: m})); err != nil {
			log.Printf("log err: %v", err)
		}
		return line
	}
}

func Handler() websocket.Handler {
	status := func() string { return fmt.Sprintf("%08b", rand.Intn(256)) }

	return func(wc *websocket.Conn) {
		signal := socket{wc, false}
		done := make(chan struct{})

		q := signal.Request().URL.Query()

		flip := q.Get("flip_offer_side") == "true"
		//iceLite := q.Get("ice_lite") == "true"
		testNat := q.Get("test_nat") == "true"

		_log := remoteLogger(&signal)

		logger := pion.CustomLoggerFactory{Log: _log}

		if testNat {
			stun.Main(logger.NewLogger("stun"))
		}

		p2p, err := pion.NewPeerConnection(logger)
		if err != nil {
			panic(err)
		}

		if flip {
			dc, err := p2p.CreateDataChannel("data")
			if err != nil {
				panic(err)
			}

			dc.OnOpen(func() {
				_ = dc.SendText(status())
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						_log("sys", "STOP TICKER!")
						return
					case _ = <-ticker.C:
						_ = dc.SendText(status())
					}
				}
			})
		}

		p2p.OnIceCandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}
			if err := signal.send(api.NewIce(*c)); err != nil {
				panic(err)
			}
		})

		p2p.OnIceConnectionStateChange(func(state webrtc.ICEConnectionState) { _log("ice", "→ %s", state) })
		p2p.OnConnectionStateChange(func(state webrtc.PeerConnectionState) { _log("rtc", "→ %s", state) })
		p2p.OnIceGatheringStateChange(func(state webrtc.ICEGathererState) { _log("ice", "→ %s", state) })
		p2p.OnSignalingStateChange(func(state webrtc.SignalingState) { _log("sig", "→ %s", state) })

		p2p.OnDataChannel(func(d *pion.DataChannel) {
			d.OnOpen(func() {
				_ = d.SendText(status())
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-done:
						_log("sys", "STOP TICKER!")
						return
					case _ = <-ticker.C:
						_ = d.SendText(status())
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
			case api.WebrtcAnswer,
				api.WebrtcOffer:
				if sdp, err := api.NewSessionDescription(m.Payload); err == nil {
					if err = p2p.SetRemoteSDP(sdp.SessionDescription); err != nil {
						_log("rtc", "err: %v", err)
						return
					}
				}
				if m.T == api.WebrtcAnswer {
					continue
				}
				answer, err := p2p.CreateAnswer()
				if err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if err = signal.send(api.NewAnswer(*answer)); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case api.WebrtcIce:
				if candidate, err := api.NewIceCandidateInit(m.Payload); err == nil {
					if err = p2p.AddIceCandidate(candidate); err != nil {
						_log("ice", "err: %v", err)
						return
					}
				}
			case api.WebrtcWaitingOffer:
				offer, err := p2p.CreateOffer()
				if err != nil {
					_log("rtc", "err: %v", err)
					return
				}
				if err = signal.send(api.NewOffer(*offer)); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case api.WebrtcClose:
				_log("sig", "!close")
				err := p2p.Close()
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
