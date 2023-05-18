package signal

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/sergystepanov/webrtc-troubleshooting/v2/internal/api"
	"github.com/sergystepanov/webrtc-troubleshooting/v2/internal/stun"
	"github.com/sergystepanov/webrtc-troubleshooting/v2/internal/webrtc"
	"golang.org/x/net/websocket"
)

type socket struct {
	*websocket.Conn
	closed bool
}

func (s *socket) close() { s.closed = true }
func (s *socket) ended(err error) bool {
	if errors.Is(err, io.EOF) {
		if err := s.Conn.Close(); err != nil {
			log.Printf("error: failed signal close, %v", err)
		}
		return true
	}
	return false
}
func (s *socket) receive(m interface{}) error { return websocket.JSON.Receive(s.Conn, m) }
func (s *socket) send(m interface{}) error {
	if s.closed {
		return nil
	}
	return websocket.JSON.Send(s.Conn, m)
}

func remoteLogger(s *socket) webrtc.LogFn {
	return func(tag string, format string, v ...any) string {
		m := fmt.Sprintf(format, v...)
		line := fmt.Sprintf("%s %s", tag, m)
		log.Printf(line)
		if !s.closed {
			if err := s.send(api.NewLog(api.Log{Tag: tag, Text: m})); err != nil {
				log.Printf("log [%v] err: %v", line, err)
			}
		}
		return line
	}
}

func logState[T webrtc.State](tag string, l webrtc.LogFn) func(state T) {
	return func(state T) { l(tag, "→ %s", state) }
}

func Handler() websocket.Handler {
	status := func() string { return fmt.Sprintf("%08b", rand.Intn(256)) }

	sendGarbage := func(d *webrtc.DataChannel, done chan struct{}) func() {
		return func() {
			_ = d.SendText(status())
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					_ = d.SendText(status())
				}
			}
		}
	}

	return func(wc *websocket.Conn) {
		signal := socket{wc, false}
		done := make(chan struct{})

		q := signal.Request().URL.Query()

		disableInterceptors := q.Get("disable_interceptors") == "true"
		flip := q.Get("flip_offer_side") == "true"
		iceServers := strings.Split(q.Get("ice_servers"), ",")
		logLevel := q.Get("log_level")
		port := q.Get("port")
		testNat := q.Get("test_nat") == "true"
		nat1to1 := q.Get("nat1to1")
		ssl := q.Get("ssl") == "true"

		_log := remoteLogger(&signal)
		logger := webrtc.NewLoggerFactory(logLevel, _log)
		_log("sys", "log level is %v", logger.Level)
		_log("sys", "secure? %v", ssl)

		if testNat {
			stun.Main(logger.NewLogger("stun"))
		}

		p2p, err := webrtc.NewPeerConnection(iceServers, disableInterceptors, port, nat1to1, logger)
		if err != nil {
			_log("sys", "fail: %v", err)
			return
		}

		if flip {
			dc, err := p2p.CreateDataChannel("data")
			if err != nil {
				panic(err)
			}
			dc.OnOpen(sendGarbage(dc, done))
		}

		p2p.OnIceCandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}
			if err := signal.send(api.NewIce(*c)); err != nil {
				_log("sys", "fail: %v", err)
			}
		})

		p2p.OnIceConnectionStateChange(logState[webrtc.ICEConnectionState]("ice", _log))
		p2p.OnConnectionStateChange(logState[webrtc.PeerConnectionState]("rtc", _log))
		p2p.OnIceGatheringStateChange(logState[webrtc.ICEGathererState]("ice", _log))
		p2p.OnSignalingStateChange(logState[webrtc.SignalingState]("sig", _log))

		p2p.OnDataChannel(func(d *webrtc.DataChannel) { d.OnOpen(sendGarbage(d, done)) })

		defer func() {
			// !to wait for message drain
			done <- struct{}{}
			signal.close()
		}()

		for {
			var m api.Message
			if err := signal.receive(&m); signal.ended(err) {
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
				if err = signal.send(api.NewSDP(*answer, api.WebrtcAnswer)); err != nil {
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
				if err = signal.send(api.NewSDP(*offer, api.WebrtcOffer)); err != nil {
					_log("rtc", "err: %v", err)
					return
				}
			case api.WebrtcClose:
				_log("sig", "!close")
				err := p2p.Close()
				if err != nil {
					log.Printf("close err: %v", err)
				}
				if err := signal.send(api.NewClose()); err != nil {
					_log("sig", "err: %v", err)
				}
			default:
				_log("sys", "err: unknown message [%v]", m.T)
				return
			}
		}
	}
}
