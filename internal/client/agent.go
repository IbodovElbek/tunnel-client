package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IbodovElbek/tunnel-client/internal/proto"
	"github.com/hashicorp/yamux"
)

// Agent owns local tunnel targets and services incoming data streams.
type Agent struct {
	mu            sync.RWMutex
	targets       map[string]string
	inspectorOnce sync.Once
	requests      []RequestRecord
	requestSeq    uint64
}

func NewAgent() *Agent { return &Agent{targets: make(map[string]string)} }
func (a *Agent) Add(tunnelID, local string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.targets[tunnelID] = local
}

type TunnelStatus struct {
	ID    string `json:"id"`
	Local string `json:"local"`
}
type RequestRecord struct {
	ID            uint64    `json:"id"`
	Time          time.Time `json:"time"`
	TunnelID      string    `json:"tunnel_id"`
	Kind          string    `json:"kind"`
	Host          string    `json:"host,omitempty"`
	RemoteAddr    string    `json:"remote_addr,omitempty"`
	RequestBytes  int64     `json:"request_bytes"`
	ResponseBytes int64     `json:"response_bytes"`
	Method        string    `json:"method,omitempty"`
	Path          string    `json:"path,omitempty"`
	Local         string    `json:"-"`
	Replay        []byte    `json:"-"`
}

func (a *Agent) Tunnels() []TunnelStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ret := make([]TunnelStatus, 0, len(a.targets))
	for id, local := range a.targets {
		ret = append(ret, TunnelStatus{ID: id, Local: local})
	}
	return ret
}
func (a *Agent) Requests() []RequestRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]RequestRecord(nil), a.requests...)
}
func (a *Agent) record(record RequestRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requestSeq++
	record.ID = a.requestSeq
	a.requests = append(a.requests, record)
	if len(a.requests) > 100 {
		a.requests = append([]RequestRecord(nil), a.requests[len(a.requests)-100:]...)
	}
}
func (a *Agent) request(id uint64) (RequestRecord, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, r := range a.requests {
		if r.ID == id {
			return r, true
		}
	}
	return RequestRecord{}, false
}

// StartInspector exposes local-only tunnel state. It never listens publicly.
func (a *Agent) StartInspector() {
	a.inspectorOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/tunnels", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(a.Tunnels())
		})
		mux.HandleFunc("/api/requests", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(a.Requests())
		})
		mux.HandleFunc("/api/requests/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/replay") {
				http.NotFound(w, r)
				return
			}
			idText := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/requests/"), "/replay")
			id, err := strconv.ParseUint(idText, 10, 64)
			if err != nil {
				http.Error(w, "noto'g'ri request ID", http.StatusBadRequest)
				return
			}
			record, ok := a.request(id)
			if !ok || record.Kind != "http" || len(record.Replay) == 0 {
				http.Error(w, "replay uchun HTTP preview topilmadi", http.StatusNotFound)
				return
			}
			conn, err := net.DialTimeout("tcp", record.Local, 5*time.Second)
			if err != nil {
				http.Error(w, "local servis ochiq emas: "+err.Error(), http.StatusBadGateway)
				return
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
			if _, err = conn.Write(record.Replay); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			_, _ = io.CopyN(w, conn, 4096)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "<html><body><h1>Shlyuz inspector</h1><p><a href='/api/tunnels'>Tunnellar JSON</a></p></body></html>")
		})
		go func() {
			if err := http.ListenAndServe("127.0.0.1:4040", mux); err != nil && err != http.ErrServerClosed {
				log.Printf("inspector ishga tushmadi: %v", err)
			}
		}()
	})
}
func (a *Agent) Accept(session *yamux.Session) error {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return err
		}
		go a.handle(stream)
	}
}
func (a *Agent) handle(stream net.Conn) {
	defer stream.Close()
	header, err := proto.Read(stream)
	if err != nil || header.Type != "data" {
		return
	}
	a.mu.RLock()
	local := a.targets[header.TunnelID]
	a.mu.RUnlock()
	if local == "" {
		return
	}
	target, err := net.Dial("tcp", local)
	if err != nil {
		log.Printf("local dial xato: %s: %v", local, err)
		if header.Kind == "http" {
			body := "local servis ochiq emas: " + local
			_, _ = fmt.Fprintf(stream, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
		}
		return
	}
	log.Printf("local ulandi: %s", local)
	defer target.Close()
	configureTCP(target)
	type copyResult struct {
		n        int64
		response bool
	}
	done := make(chan copyResult, 2)
	preview := &limitedCapture{limit: 4096}
	go func() {
		n, err := io.Copy(target, io.TeeReader(stream, preview))
		log.Printf("local'ga yozildi: %d bayt; so'rov yo'nalishi tugadi (%d bayt, %v)", n, n, err)
		closeWrite(target)
		done <- copyResult{n: n}
	}()
	go func() {
		n, err := io.Copy(stream, target)
		log.Printf("local'dan o'qildi: %d bayt; javob yo'nalishi tugadi (%d bayt, %v)", n, n, err)
		_ = stream.Close()
		done <- copyResult{n: n, response: true}
	}()
	first, second := <-done, <-done
	request, response := first.n, second.n
	if first.response {
		request, response = second.n, first.n
	}
	method, path := requestLine(preview.Bytes())
	a.record(RequestRecord{Time: time.Now().UTC(), TunnelID: header.TunnelID, Kind: header.Kind, Host: header.Host, RemoteAddr: header.RemoteAddr, RequestBytes: request, ResponseBytes: response, Method: method, Path: path, Local: local, Replay: preview.Bytes()})
}

type limitedCapture struct {
	limit int
	b     []byte
}

func (c *limitedCapture) Write(p []byte) (int, error) {
	if len(c.b) < c.limit {
		n := c.limit - len(c.b)
		if n > len(p) {
			n = len(p)
		}
		c.b = append(c.b, p[:n]...)
	}
	return len(p), nil
}
func (c *limitedCapture) Bytes() []byte { return append([]byte(nil), c.b...) }
func requestLine(raw []byte) (string, string) {
	first := strings.SplitN(string(raw), "\r\n", 2)[0]
	fields := strings.Fields(first)
	if len(fields) >= 2 {
		return fields[0], fields[1]
	}
	return "", ""
}

func configureTCP(conn net.Conn) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcp.SetNoDelay(true)
	_ = tcp.SetKeepAlive(true)
	_ = tcp.SetKeepAlivePeriod(30 * time.Second)
}

func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
		return
	}
	_ = conn.Close()
}
