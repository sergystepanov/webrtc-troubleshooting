package signal

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/pion/logging"
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

func (s *socket) close() { s.closed = true }
func (s *socket) ended(err error) bool {
	if errors.Is(err, io.EOF) {
		if err := s.WriteClose(1000); err != nil {
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

	sendGarbage := func(d *pion.DataChannel, done chan struct{}) func() {
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
		logLevel := q.Get("log_level")
		testNat := q.Get("test_nat") == "true"
		iceServers := q.Get("ice_servers")
		port := q.Get("port")

		_log := remoteLogger(&signal)

		logger := pion.CustomLoggerFactory{
			Level: logging.LogLevelTrace,
			Log:   _log,
		}
		if logLevel != "" {
			if l, err := strconv.Atoi(logLevel); err == nil {
				logger.Level = logging.LogLevel(l)
			}
			_log("sys", "log level is %v", logger.Level)
		}

		if testNat {
			stun.Main(logger.NewLogger("stun"))
		}

		p2p, err := pion.NewPeerConnection(strings.Split(iceServers, ","), disableInterceptors, port, logger)
		if err != nil {
			panic(err)
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
				panic(err)
			}
		})

		p2p.OnIceConnectionStateChange(func(state webrtc.ICEConnectionState) { _log("ice", "→ %s", state) })
		p2p.OnConnectionStateChange(func(state webrtc.PeerConnectionState) { _log("rtc", "→ %s", state) })
		p2p.OnIceGatheringStateChange(func(state webrtc.ICEGathererState) { _log("ice", "→ %s", state) })
		p2p.OnSignalingStateChange(func(state webrtc.SignalingState) { _log("sig", "→ %s", state) })

		p2p.OnDataChannel(func(d *pion.DataChannel) { d.OnOpen(sendGarbage(d, done)) })

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
