package sip

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type SIPServer struct {
	listenAddr   string
	asteriskAddr string
	logger       zerolog.Logger
	conn         *net.UDPConn
	mutex        sync.RWMutex
	running      bool
}

type SIPServerConfig struct {
	ListenAddr   string
	AsteriskAddr string
	Logger       zerolog.Logger
}

type CallRequest struct {
	Destination string
	CallerID    string
	UUI         string
	CallID      string
}

func NewSIPServer(config SIPServerConfig) *SIPServer {
	return &SIPServer{
		listenAddr:   config.ListenAddr,
		asteriskAddr: config.AsteriskAddr,
		logger:       config.Logger,
	}
}

func (s *SIPServer) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return nil
	}

	s.logger.Info().
		Str("listen_addr", s.listenAddr).
		Str("asterisk_addr", s.asteriskAddr).
		Msg("Starting SIP server emulator")

	// Create UDP listener (optional - mainly for receiving responses)
	if s.listenAddr != "" {
		addr, err := net.ResolveUDPAddr("udp", s.listenAddr)
		if err != nil {
			return fmt.Errorf("failed to resolve listen address: %w", err)
		}

		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			return fmt.Errorf("failed to create UDP listener: %w", err)
		}

		s.conn = conn
	}

	s.running = true
	s.logger.Info().Msg("SIP server emulator started successfully")

	return nil
}

func (s *SIPServer) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info().Msg("Stopping SIP server emulator")

	if s.conn != nil {
		s.conn.Close()
	}

	s.running = false
	s.logger.Info().Msg("SIP server emulator stopped")

	return nil
}

func (s *SIPServer) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// EmulateIncomingCall sends a SIP INVITE to Asterisk to emulate an incoming call
func (s *SIPServer) EmulateIncomingCall(req CallRequest) error {
	s.logger.Info().
		Str("destination", req.Destination).
		Str("caller_id", req.CallerID).
		Str("uui", req.UUI).
		Str("call_id", req.CallID).
		Msg("Emulating incoming SIP call")

	// Parse Asterisk address
	asteriskHost, asteriskPort, err := net.SplitHostPort(s.asteriskAddr)
	if err != nil {
		return fmt.Errorf("failed to parse Asterisk address: %w", err)
	}

	// Generate unique identifiers
	tag := generateTag()
	branch := generateBranch()

	// Build SIP INVITE message
	var sipMessage strings.Builder

	// Request line
	sipMessage.WriteString(fmt.Sprintf("INVITE sip:%s@%s:%s SIP/2.0\r\n", req.Destination, asteriskHost, asteriskPort))

	// Via header (critical - shows where response should go)
	sipMessage.WriteString(fmt.Sprintf("Via: SIP/2.0/UDP %s;branch=%s\r\n", s.listenAddr, branch))

	// From header (external caller - critical for banking systems)
	sipMessage.WriteString(fmt.Sprintf("From: \"%s\" <sip:%s@trunk-provider.com>;tag=%s\r\n", req.CallerID, req.CallerID, tag))

	// To header (destination)
	sipMessage.WriteString(fmt.Sprintf("To: <sip:%s@%s:%s>\r\n", req.Destination, asteriskHost, asteriskPort))

	// Call-ID header
	sipMessage.WriteString(fmt.Sprintf("Call-ID: %s\r\n", req.CallID))

	// CSeq header
	sipMessage.WriteString("CSeq: 1 INVITE\r\n")

	// Contact header
	sipMessage.WriteString(fmt.Sprintf("Contact: <sip:%s@%s>\r\n", req.CallerID, s.listenAddr))

	// Max-Forwards header
	sipMessage.WriteString("Max-Forwards: 70\r\n")

	// P-Asserted-Identity header (CRITICAL for banking systems)
	sipMessage.WriteString(fmt.Sprintf("P-Asserted-Identity: <sip:%s@trunk-provider.com>\r\n", req.CallerID))

	// User-to-User header for UUI (CRITICAL FIX - UUI in SIP headers, not variables)
	if req.UUI != "" {
		sipMessage.WriteString(fmt.Sprintf("User-to-User: %s\r\n", req.UUI))
	}

	// Content-Type and Content-Length (minimal SDP)
	sdpBody := fmt.Sprintf("v=0\r\no=- %d 1 IN IP4 127.0.0.1\r\ns=TheLoopA\r\nc=IN IP4 127.0.0.1\r\nt=0 0\r\nm=audio 8000 RTP/AVP 0\r\na=rtpmap:0 PCMU/8000\r\n", time.Now().Unix())
	sipMessage.WriteString("Content-Type: application/sdp\r\n")
	sipMessage.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(sdpBody)))

	// Empty line before body
	sipMessage.WriteString("\r\n")

	// SDP body
	sipMessage.WriteString(sdpBody)

	// Create UDP connection to Asterisk
	conn, err := net.Dial("udp", s.asteriskAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to Asterisk: %w", err)
	}
	defer conn.Close()

	// Send the INVITE
	inviteMessage := sipMessage.String()
	s.logger.Debug().
		Str("sip_message", inviteMessage).
		Msg("Sending raw SIP INVITE to Asterisk")

	if _, err := conn.Write([]byte(inviteMessage)); err != nil {
		return fmt.Errorf("failed to send INVITE: %w", err)
	}

	s.logger.Info().
		Str("call_id", req.CallID).
		Str("destination", req.Destination).
		Str("asterisk_addr", s.asteriskAddr).
		Msg("SIP INVITE sent successfully to Asterisk")

	return nil
}

// Helper functions
func generateTag() string {
	return fmt.Sprintf("tag-%d", time.Now().UnixNano())
}

func generateBranch() string {
	return fmt.Sprintf("z9hG4bK-%d", time.Now().UnixNano())
}
