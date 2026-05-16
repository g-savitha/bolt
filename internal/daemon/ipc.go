package daemon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

// IPC message types — the set of commands the CLI can send to the daemon.
const (
	CmdStatus    = "status"
	CmdListPeers = "list_peers"
	CmdDaemonID  = "id"
)

// IPCRequest is sent by the CLI to the daemon over the Unix socket.
type IPCRequest struct {
	// ID is a client-chosen string echoed back in the response, allowing
	// the CLI to match responses to requests if it sends multiple concurrently.
	ID      string          `json:"id"`
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// IPCResponse is sent by the daemon back to the CLI.
type IPCResponse struct {
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// StatusPayload is the response payload for the CmdStatus command.
type StatusPayload struct {
	Fingerprint string            `json:"fingerprint"`
	Nickname    string            `json:"nickname"`
	Port        int               `json:"port"`
	OnlinePeers []OnlinePeerEntry `json:"online_peers"`
}

// OnlinePeerEntry is one row in the StatusPayload peer list.
type OnlinePeerEntry struct {
	Fingerprint string `json:"fingerprint"`
	Nickname    string `json:"nickname"`
}

// IPCServer listens on a Unix socket and handles requests from CLI processes.
type IPCServer struct {
	ln     net.Listener
	daemon *Daemon
	wg     sync.WaitGroup
}

// NewIPCServer creates and starts an IPC server on socketPath.
// The caller must call Close() when done.
func NewIPCServer(socketPath string, d *Daemon) (*IPCServer, error) {
	// Remove a stale socket file from a previous unclean shutdown.
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale socket file: %w", err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on unix socket %s: %w", socketPath, err)
	}

	s := &IPCServer{ln: ln, daemon: d}
	go s.acceptLoop()
	return s, nil
}

// Close stops the IPC server and waits for all in-flight requests to finish.
func (s *IPCServer) Close() {
	s.ln.Close()
	s.wg.Wait()
}

func (s *IPCServer) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

// handleConn serves a single CLI connection: read one request, write one response.
func (s *IPCServer) handleConn(conn net.Conn) {
	defer conn.Close()

	var req IPCRequest
	if err := readIPCFrame(conn, &req); err != nil {
		return // client disconnected before sending a full request
	}

	resp := s.dispatch(req)
	resp.ID = req.ID

	if err := writeIPCFrame(conn, resp); err != nil {
		fmt.Fprintf(os.Stderr, "ipc: write response to cli: %v\n", err)
	}
}

// dispatch routes an IPCRequest to the appropriate handler and returns a response.
func (s *IPCServer) dispatch(req IPCRequest) IPCResponse {
	switch req.Command {
	case CmdStatus:
		return s.handleStatus()
	case CmdListPeers:
		return s.handleListPeers()
	case CmdDaemonID:
		return s.handleID()
	default:
		return errorResponse(fmt.Sprintf("unknown command %q — update flick to use this feature", req.Command))
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

func (s *IPCServer) handleListPeers() IPCResponse {
	return s.handleStatus() // status includes peer list; reuse for now
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

// --- IPCClient ---

// IPCClient connects to a running daemon's Unix socket and sends commands.
type IPCClient struct {
	conn net.Conn
}

// Connect opens a connection to the daemon's Unix socket.
// Returns an error if the daemon is not running.
func Connect(socketPath string) (*IPCClient, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf(
			"connect to flick daemon: %w\n  Is the daemon running? Try: flick daemon",
			err,
		)
	}
	return &IPCClient{conn: conn}, nil
}

// Send sends a command to the daemon and returns the response.
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
		ID:      "1", // single request per connection — ID is trivial
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

// Close closes the connection to the daemon.
func (c *IPCClient) Close() error {
	return c.conn.Close()
}

// --- framing helpers ---
// IPC frames use the same length-prefix format as wire.go:
// [4-byte big-endian uint32 length][JSON bytes]

func writeIPCFrame(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal ipc message: %w", err)
	}

	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(data)))
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
	if length > 1<<20 { // 1 MB cap on IPC messages
		return fmt.Errorf("ipc frame too large: %d bytes", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// --- response helpers ---

func okResponse(payload any) IPCResponse {
	data, _ := json.Marshal(payload)
	return IPCResponse{OK: true, Payload: data}
}

func errorResponse(msg string) IPCResponse {
	return IPCResponse{OK: false, Error: msg}
}
