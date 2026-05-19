package daemon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
)

// IPC commands — extend this set as features grow.
const (
	CmdStatus    = "status"
	CmdListPeers = "list_peers"
	CmdDaemonID  = "id"
	CmdConnect   = "connect"
	CmdSendChat  = "send_chat"
	CmdSubscribe = "subscribe"
)

// IPCRequest is sent by the CLI to the daemon.
type IPCRequest struct {
	ID      string          `json:"id"`
	Token   string          `json:"token"`
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// IPCResponse is sent by the daemon for request/response commands.
type IPCResponse struct {
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// IPCEvent is pushed to subscribed CLI clients (e.g. incoming chat).
type IPCEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ChatEventPayload is the payload for a "chat" IPC event.
type ChatEventPayload struct {
	From        string `json:"from"`
	Fingerprint string `json:"fingerprint"`
	Body        string `json:"body"`
	SentAt      string `json:"sent_at"`
}

// StatusPayload is the response payload for CmdStatus.
type StatusPayload struct {
	Fingerprint string            `json:"fingerprint"`
	Nickname    string            `json:"nickname"`
	Port        int               `json:"port"`
	OnlinePeers []OnlinePeerEntry `json:"online_peers"`
}

// OnlinePeerEntry is one row in the peer list.
type OnlinePeerEntry struct {
	Fingerprint string `json:"fingerprint"`
	Nickname    string `json:"nickname"`
}

// ConnectPayload is the request body for CmdConnect.
type ConnectPayload struct {
	Address string `json:"address"`
}

// SendChatPayload is the request body for CmdSendChat.
type SendChatPayload struct {
	Peer string `json:"peer"`
	Body string `json:"body"`
}

// IPCServer handles local CLI connections.
type IPCServer struct {
	ln     net.Listener
	daemon *Daemon
	token  string
	wg     sync.WaitGroup
	subsMu sync.Mutex
	subs   map[net.Conn]struct{}
}

// NewIPCServer starts the platform IPC listener.
func NewIPCServer(configDir string, d *Daemon) (*IPCServer, error) {
	token, err := loadOrCreateIPCToken(configDir)
	if err != nil {
		return nil, err
	}

	ln, err := listenIPC(configDir)
	if err != nil {
		return nil, err
	}

	s := &IPCServer{
		ln:     ln,
		daemon: d,
		token:  token,
		subs:   make(map[net.Conn]struct{}),
	}
	go s.acceptLoop()
	return s, nil
}

// Close stops the IPC server and waits for all handlers to exit.
// Active subscribe connections are closed first so handleSubscribe
// unblocks from conn.Read (fixes shutdown hang when a CLI is attached).
func (s *IPCServer) Close() {
	s.ln.Close()
	s.closeSubscriberConns()
	s.wg.Wait()
}

func (s *IPCServer) closeSubscriberConns() {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for conn := range s.subs {
		_ = conn.Close()
	}
	s.subs = make(map[net.Conn]struct{})
}

// PublishChat delivers a chat event to all subscribed CLI clients.
func (s *IPCServer) PublishChat(evt ChatEventPayload) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	event := IPCEvent{Type: "chat", Payload: data}
	s.subsMu.Lock()
	conns := make([]net.Conn, 0, len(s.subs))
	for conn := range s.subs {
		conns = append(conns, conn)
	}
	s.subsMu.Unlock()

	for _, conn := range conns {
		if err := writeIPCFrame(conn, event); err != nil {
			s.subsMu.Lock()
			delete(s.subs, conn)
			s.subsMu.Unlock()
			_ = conn.Close()
		}
	}
}

func (s *IPCServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *IPCServer) handleConn(conn net.Conn) {
	defer conn.Close()

	var req IPCRequest
	if err := readIPCFrame(conn, &req); err != nil {
		return
	}
	if req.Token != s.token {
		_ = writeIPCFrame(conn, errorResponse("invalid ipc token"))
		return
	}

	if req.Command == CmdSubscribe {
		s.handleSubscribe(conn)
		return
	}

	resp := s.dispatch(req)
	resp.ID = req.ID
	_ = writeIPCFrame(conn, resp)
}

func (s *IPCServer) handleSubscribe(conn net.Conn) {
	s.subsMu.Lock()
	s.subs[conn] = struct{}{}
	s.subsMu.Unlock()

	defer func() {
		s.subsMu.Lock()
		delete(s.subs, conn)
		s.subsMu.Unlock()
		conn.Close()
	}()

	// Hold connection open until the client disconnects.
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}

func (s *IPCServer) dispatch(req IPCRequest) IPCResponse {
	switch req.Command {
	case CmdStatus, CmdListPeers:
		return s.handleStatus()
	case CmdDaemonID:
		return s.handleID()
	case CmdConnect:
		return s.handleConnect(req.Payload)
	case CmdSendChat:
		return s.handleSendChat(req.Payload)
	default:
		return errorResponse(fmt.Sprintf("unknown command %q — update bolt to use this feature", req.Command))
	}
}

func (s *IPCServer) handleStatus() IPCResponse {
	online := s.daemon.registry.Online()
	entries := make([]OnlinePeerEntry, len(online))
	for i, pc := range online {
		entries[i] = OnlinePeerEntry{
			Fingerprint: pc.PeerFingerprint(),
			Nickname:    pc.PeerNickname(),
		}
	}
	payload := StatusPayload{
		Fingerprint: s.daemon.id.Fingerprint(),
		Nickname:    s.daemon.cfg.Nickname,
		Port:        s.daemon.cfg.Port,
		OnlinePeers: entries,
	}
	return okResponse(payload)
}

func (s *IPCServer) handleID() IPCResponse {
	type idPayload struct {
		Fingerprint string `json:"fingerprint"`
		Nickname    string `json:"nickname"`
	}
	return okResponse(idPayload{
		Fingerprint: s.daemon.id.Fingerprint(),
		Nickname:    s.daemon.cfg.Nickname,
	})
}

func (s *IPCServer) handleConnect(raw json.RawMessage) IPCResponse {
	var p ConnectPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return errorResponse("invalid connect payload")
	}
	if p.Address == "" {
		return errorResponse("connect requires an address (e.g. 192.168.1.42)")
	}
	if err := s.daemon.ConnectPeer(s.daemon.runCtx, p.Address); err != nil {
		return errorResponse(err.Error())
	}
	return okResponse(map[string]string{"address": p.Address})
}

func (s *IPCServer) handleSendChat(raw json.RawMessage) IPCResponse {
	var p SendChatPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return errorResponse("invalid send_chat payload")
	}
	if err := s.daemon.SendChat(s.daemon.runCtx, p.Peer, p.Body); err != nil {
		return errorResponse(err.Error())
	}
	return okResponse(map[string]string{"peer": p.Peer})
}

// IPCClient talks to the daemon.
type IPCClient struct {
	conn      net.Conn
	configDir string
	token     string
}

// Connect opens an authenticated connection to the daemon.
func Connect(configDir string) (*IPCClient, error) {
	token, err := readIPCToken(configDir)
	if err != nil {
		return nil, err
	}
	conn, err := dialIPC(configDir)
	if err != nil {
		return nil, err
	}
	return &IPCClient{conn: conn, configDir: configDir, token: token}, nil
}

// Send runs one request/response round trip.
func (c *IPCClient) Send(command string, payload any) (*IPCResponse, error) {
	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal ipc payload: %w", err)
		}
		rawPayload = data
	}

	req := IPCRequest{
		ID:      "1",
		Token:   c.token,
		Command: command,
		Payload: rawPayload,
	}
	if err := writeIPCFrame(c.conn, req); err != nil {
		return nil, fmt.Errorf("send ipc request: %w", err)
	}

	var resp IPCResponse
	if err := readIPCFrame(c.conn, &resp); err != nil {
		return nil, fmt.Errorf("read ipc response: %w", err)
	}
	return &resp, nil
}

// Close closes the connection.
func (c *IPCClient) Close() error {
	return c.conn.Close()
}

// Subscribe opens a long-lived connection that receives IPC events.
func Subscribe(configDir string) (conn net.Conn, token string, err error) {
	token, err = readIPCToken(configDir)
	if err != nil {
		return nil, "", err
	}
	conn, err = dialIPC(configDir)
	if err != nil {
		return nil, "", err
	}

	req := IPCRequest{
		ID:      "sub",
		Token:   token,
		Command: CmdSubscribe,
	}
	if err := writeIPCFrame(conn, req); err != nil {
		conn.Close()
		return nil, "", err
	}
	return conn, token, nil
}

// ReadEvent reads the next event frame from a subscribe connection.
func ReadEvent(conn net.Conn) (*IPCEvent, error) {
	var evt IPCEvent
	if err := readIPCFrame(conn, &evt); err != nil {
		return nil, err
	}
	return &evt, nil
}

func writeIPCFrame(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal ipc message: %w", err)
	}
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(data))) //nolint:gosec // G115: message size is always well below MaxUint32
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

func readIPCFrame(r io.Reader, dst any) error {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(buf[:])
	if length > 1<<20 {
		return fmt.Errorf("ipc frame too large: %d bytes", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func okResponse(payload any) IPCResponse {
	data, _ := json.Marshal(payload)
	return IPCResponse{OK: true, Payload: data}
}

func errorResponse(msg string) IPCResponse {
	return IPCResponse{OK: false, Error: msg}
}
