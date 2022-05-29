package stun

import (
	"errors"
	"net"
	"time"

	"github.com/pion/logging"
	"github.com/pion/stun"
)

// This stuff is based on Pion's STUN tester (https://github.com/pion/stun/tree/master/cmd/stun-nat-behaviour),
// it implements RFC5780's tests:
// - 4.3.  Determining NAT Mapping Behavior
// - 4.4.  Determining NAT Filtering Behavior

type (
	stunServerConn struct {
		conn        net.PacketConn
		LocalAddr   net.Addr
		RemoteAddr  *net.UDPAddr
		OtherAddr   *net.UDPAddr
		messageChan chan *stun.Message
	}
	stunData struct {
		xorAddr    *stun.XORMappedAddress
		otherAddr  *stun.OtherAddress
		respOrigin *stun.ResponseOrigin
		mappedAddr *stun.MappedAddress
		software   *stun.Software
	}
)

func (c *stunServerConn) Close() error {
	return c.conn.Close()
}

var log logging.LeveledLogger

const (
	stunAddr = "stun.nextcloud.com:443"
	// the number of seconds to wait for STUN server's response
	timeout = 3 * time.Second
)

var (
	errResponseMessage = errors.New("error reading from response message channel")
	errTimedOut        = errors.New("timed out waiting for response")
	errNoOtherAddress  = errors.New("no OTHER-ADDRESS in message")
)

func Main(l logging.LeveledLogger) {
	log = l
	//logging.NewDefaultLeveledLoggerForScope("", logging.LogLevelDebug, os.Stdout)
	if err := mappingTests(stunAddr); err != nil {
		log.Warn("NAT mapping behavior: inconclusive")
	}
	if err := filteringTests(stunAddr); err != nil {
		log.Warn("NAT filtering behavior: inconclusive")
	}
}

// RFC5780: 4.3.  Determining NAT Mapping Behavior
func mappingTests(addrStr string) error {
	mapTestConn, err := connect(addrStr)
	if err != nil {
		log.Warnf("Error creating STUN connection: %s\n", err.Error())
		return err
	}

	// Test I: Regular binding request
	log.Info("Mapping Test I: Regular binding request")
	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	resp, err := mapTestConn.roundTrip(request, mapTestConn.RemoteAddr)
	if err != nil {
		return err
	}

	// Parse response message for XOR-MAPPED-ADDRESS and make sure OTHER-ADDRESS valid
	stun1 := parse(resp)
	if stun1.xorAddr == nil || stun1.otherAddr == nil {
		log.Info("Error: NAT discovery feature not supported by this server")
		return errNoOtherAddress
	}
	addr, err := net.ResolveUDPAddr("udp4", stun1.otherAddr.String())
	if err != nil {
		log.Infof("Failed resolving OTHER-ADDRESS: %v\n", stun1.otherAddr)
		return err
	}
	mapTestConn.OtherAddr = addr
	log.Infof("Received XOR-MAPPED-ADDRESS: %v\n", stun1.xorAddr)

	// Assert mapping behavior
	if stun1.xorAddr.String() == mapTestConn.LocalAddr.String() {
		log.Warn("=> NAT mapping behavior: endpoint independent (no NAT)")
		return nil
	}

	// Test II: Send binding request to the other address but primary port
	log.Info("Mapping Test II: Send binding request to the other address but primary port")
	otherAddr := *mapTestConn.OtherAddr
	otherAddr.Port = mapTestConn.RemoteAddr.Port
	resp, err = mapTestConn.roundTrip(request, &otherAddr)
	if err != nil {
		return err
	}

	// Assert mapping behavior
	stun2 := parse(resp)
	log.Infof("Received XOR-MAPPED-ADDRESS: %v\n", stun2.xorAddr)
	if stun2.xorAddr.String() == stun1.xorAddr.String() {
		log.Warn("=> NAT mapping behavior: endpoint independent")
		return nil
	}

	// Test III: Send binding request to the other address and port
	log.Info("Mapping Test III: Send binding request to the other address and port")
	resp, err = mapTestConn.roundTrip(request, mapTestConn.OtherAddr)
	if err != nil {
		return err
	}

	// Assert mapping behavior
	stun3 := parse(resp)
	log.Infof("Received XOR-MAPPED-ADDRESS: %v\n", stun3.xorAddr)
	if stun3.xorAddr.String() == stun2.xorAddr.String() {
		log.Warn("=> NAT mapping behavior: address dependent")
	} else {
		log.Warn("=> NAT mapping behavior: address and port dependent")
	}

	return mapTestConn.Close()
}

// RFC5780: 4.4.  Determining NAT Filtering Behavior
func filteringTests(addrStr string) error {
	mapTestConn, err := connect(addrStr)
	if err != nil {
		log.Warnf("Error creating STUN connection: %s\n", err.Error())
		return err
	}

	// Test I: Regular binding request
	log.Info("Filtering Test I: Regular binding request")
	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	resp, err := mapTestConn.roundTrip(request, mapTestConn.RemoteAddr)
	if err != nil || errors.Is(err, errTimedOut) {
		return err
	}
	stun0 := parse(resp)
	if stun0.xorAddr == nil || stun0.otherAddr == nil {
		log.Warn("Error: NAT discovery feature not supported by this server")
		return errNoOtherAddress
	}
	addr, err := net.ResolveUDPAddr("udp4", stun0.otherAddr.String())
	if err != nil {
		log.Infof("Failed resolving OTHER-ADDRESS: %v\n", stun0.otherAddr)
		return err
	}
	mapTestConn.OtherAddr = addr

	// Test II: Request to change both IP and port
	log.Info("Filtering Test II: Request to change both IP and port")
	request = stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	request.Add(stun.AttrChangeRequest, []byte{0x00, 0x00, 0x00, 0x06})

	resp, err = mapTestConn.roundTrip(request, mapTestConn.RemoteAddr)
	if err == nil {
		parse(resp) // just to print out the resp
		log.Warn("=> NAT filtering behavior: endpoint independent")
		return nil
	} else if !errors.Is(err, errTimedOut) {
		return err // something else went wrong
	}

	// Test III: Request to change port only
	log.Info("Filtering Test III: Request to change port only")
	request = stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	request.Add(stun.AttrChangeRequest, []byte{0x00, 0x00, 0x00, 0x02})

	resp, err = mapTestConn.roundTrip(request, mapTestConn.RemoteAddr)
	if err == nil {
		parse(resp) // just to print out the resp
		log.Warn("=> NAT filtering behavior: address dependent")
	} else if errors.Is(err, errTimedOut) {
		log.Warn("=> NAT filtering behavior: address and port dependent")
	}

	return mapTestConn.Close()
}

// Parse a STUN message
func parse(msg *stun.Message) (ret stunData) {
	ret.mappedAddr = &stun.MappedAddress{}
	ret.xorAddr = &stun.XORMappedAddress{}
	ret.respOrigin = &stun.ResponseOrigin{}
	ret.otherAddr = &stun.OtherAddress{}
	ret.software = &stun.Software{}
	if ret.xorAddr.GetFrom(msg) != nil {
		ret.xorAddr = nil
	}
	if ret.otherAddr.GetFrom(msg) != nil {
		ret.otherAddr = nil
	}
	if ret.respOrigin.GetFrom(msg) != nil {
		ret.respOrigin = nil
	}
	if ret.mappedAddr.GetFrom(msg) != nil {
		ret.mappedAddr = nil
	}
	if ret.software.GetFrom(msg) != nil {
		ret.software = nil
	}
	log.Debugf(
		"%v\n"+
			"\tMAPPED-ADDRESS:     %v\n"+
			"\tXOR-MAPPED-ADDRESS: %v\n"+
			"\tRESPONSE-ORIGIN:    %v\n"+
			"\tOTHER-ADDRESS:      %v\n"+
			"\tSOFTWARE:           %v\n",
		msg, ret.mappedAddr, ret.xorAddr, ret.respOrigin, ret.otherAddr, ret.software)
	for _, attr := range msg.Attributes {
		switch attr.Type {
		case
			stun.AttrXORMappedAddress,
			stun.AttrOtherAddress,
			stun.AttrResponseOrigin,
			stun.AttrMappedAddress,
			stun.AttrSoftware:
			break
		default:
			log.Debugf("\t%v (l=%v)\n", attr, attr.Length)
		}
	}
	return ret
}

// Given an address string, returns a StunServerConn
func connect(addrStr string) (*stunServerConn, error) {
	log.Infof("connecting to STUN server: %s\n", addrStr)
	addr, err := net.ResolveUDPAddr("udp4", addrStr)
	if err != nil {
		log.Warnf("Error resolving address: %s\n", err.Error())
		return nil, err
	}

	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, err
	}
	log.Infof("Local address: %s\n", c.LocalAddr())
	log.Infof("Remote address: %s\n", addr.String())

	mChan := listen(c)

	return &stunServerConn{
		conn:        c,
		LocalAddr:   c.LocalAddr(),
		RemoteAddr:  addr,
		messageChan: mChan,
	}, nil
}

// Send request and wait for response or timeout
func (c *stunServerConn) roundTrip(msg *stun.Message, addr net.Addr) (*stun.Message, error) {
	_ = msg.NewTransactionID()
	log.Infof("Sending to %v: (%v bytes)\n", addr, msg.Length+20)
	log.Debugf("%v\n", msg)
	for _, attr := range msg.Attributes {
		log.Debugf("\t%v (l=%v)\n", attr, attr.Length)
	}
	_, err := c.conn.WriteTo(msg.Raw, addr)
	if err != nil {
		log.Warnf("Error sending request to %v\n", addr)
		return nil, err
	}

	// Wait for response or timeout
	select {
	case m, ok := <-c.messageChan:
		if !ok {
			return nil, errResponseMessage
		}
		return m, nil
	case <-time.After(timeout):
		log.Infof("Timed out waiting for response from server %v\n", addr)
		return nil, errTimedOut
	}
}

func listen(conn *net.UDPConn) chan *stun.Message {
	mess := make(chan *stun.Message)
	go func() {
		defer close(mess)
		for {
			buf := make([]byte, 1024)
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			log.Infof("Response from %v: (%v bytes)\n", addr, n)
			buf = buf[:n]
			m := new(stun.Message)
			m.Raw = buf
			if err = m.Decode(); err != nil {
				log.Infof("Error decoding message: %v\n", err)
				return
			}
			mess <- m
		}
	}()
	return mess
}
