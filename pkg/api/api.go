package api

import (
	"encoding/json"
	"time"

	"github.com/pion/webrtc/v3"
)

const (
	WebrtcIce          MessageType = "ICE"
	WebrtcOffer        MessageType = "OFFER"
	WebrtcAnswer       MessageType = "ANSWER"
	WebrtcWaitingOffer MessageType = "WAITING_OFFER"
	WebrtcClose        MessageType = "CLOSE"
	MessageLog         MessageType = "LOG"
)

type (
	Answer struct {
		typed
		Payload webrtc.SessionDescription `json:"p"`
	}
	Offer struct {
		typed
		Payload webrtc.SessionDescription `json:"p"`
	}
	ICE struct {
		typed
		Payload webrtc.ICECandidateInit `json:"p"`
	}
	Close struct {
		typed
	}
	Log struct {
		Tag  string    `json:"tag"`
		Time time.Time `json:"time"`
		Text string    `json:"text"`
	}
	LogMessage struct {
		typed
		Payload Log `json:"p"`
	}
	Message struct {
		typed
		Payload json.RawMessage `json:"p,omitempty"`
	}
	MessageType        string
	SessionDescription struct {
		webrtc.SessionDescription
	}
	typed struct {
		T MessageType `json:"t"`
	}
)

func NewSessionDescription(data []byte) (*SessionDescription, error) {
	var sess SessionDescription
	err := json.Unmarshal(data, &sess)
	return &sess, err
}

func NewIceCandidateInit(data []byte) (webrtc.ICECandidateInit, error) {
	var candidate webrtc.ICECandidateInit
	err := json.Unmarshal(data, &candidate)
	return candidate, err
}

func NewAnswer(sdp webrtc.SessionDescription) Answer {
	return Answer{typed: typed{WebrtcAnswer}, Payload: sdp}
}
func NewOffer(sdp webrtc.SessionDescription) Offer {
	return Offer{typed: typed{WebrtcOffer}, Payload: sdp}
}
func NewIce(candidate webrtc.ICECandidate) ICE {
	return ICE{typed: typed{WebrtcIce}, Payload: candidate.ToJSON()}
}
func NewLog(l Log) LogMessage {
	return LogMessage{
		typed{MessageLog},
		Log{
			Tag:  l.Tag,
			Time: time.Now(),
			Text: l.Text,
		},
	}
}

func NewClose() Close { return Close{typed{WebrtcClose}} }
