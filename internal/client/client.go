// Package client manages the tunnel client's resilient control connection.
package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/IbodovElbek/tunnel-client/internal/proto"
	"github.com/hashicorp/yamux"
)

type Config struct {
	Server, Token, Version string
	TLSConfig              *tls.Config
	Plain                  bool
}
type Client struct {
	Config      Config
	OnConnected func(*yamux.Session) error
	mu          sync.Mutex
	session     *yamux.Session
}

var errRejected = errors.New("server so'rovni rad etdi")

func newYamuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.KeepAliveInterval = 15 * time.Second
	cfg.ConnectionWriteTimeout = 10 * time.Second
	cfg.MaxStreamWindowSize = 4 << 20
	return cfg
}

type TunnelRequest struct {
	ID, Kind, Local, Subdomain, Domain string
	RemotePort                         int
}

func (c *Client) RunTunnel(ctx context.Context, request TunnelRequest, agent *Agent) error {
	agent.StartInspector()
	if !strings.Contains(request.Local, ":") {
		request.Local = "127.0.0.1:" + request.Local
	}
	delay := time.Second
	for {
		err := c.runTunnelOnce(ctx, &request, agent)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errRejected) {
			return err
		}
		if err != nil {
			log.Printf("client: tunnel uzildi: %v; qayta ulanadi", err)
		}
		jitter := time.Duration(rand.Int64N(int64(delay/3 + 1)))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay + jitter):
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
}

func (c *Client) runTunnelOnce(ctx context.Context, request *TunnelRequest, agent *Agent) error {
	conn, err := c.dialControl(ctx)
	if err != nil {
		return err
	}
	log.Print("client: yamux session")
	defer conn.Close()
	_ = conn.SetDeadline(time.Time{})
	s, err := yamux.Client(conn, newYamuxConfig())
	if err != nil {
		return fmt.Errorf("yamux session: %w", err)
	}
	defer s.Close()
	control, err := s.OpenStream()
	if err != nil {
		return fmt.Errorf("control stream ochish: %w", err)
	}
	defer control.Close()
	if err = proto.Write(control, proto.Auth(c.Config.Token, c.Config.Version, runtime.GOOS)); err != nil {
		return fmt.Errorf("auth yuborish: %w", err)
	}
	log.Print("client: -> auth")
	_ = control.SetReadDeadline(time.Now().Add(10 * time.Second))
	m, err := proto.Read(control)
	_ = control.SetReadDeadline(time.Time{})
	if err != nil {
		return fmt.Errorf("auth_ok 10s ichida kelmadi: %w", err)
	}
	log.Printf("client: <- %s", m.Type)
	if m.Type != "auth_ok" {
		return fmt.Errorf("%w: %s", errRejected, m.Reason)
	}
	if err = proto.Write(control, proto.Message{Type: "tunnel_request", ID: request.ID, Kind: request.Kind, Local: request.Local, Subdomain: request.Subdomain, Domain: request.Domain, RemotePort: request.RemotePort}); err != nil {
		return fmt.Errorf("tunnel_request yuborish: %w", err)
	}
	log.Print("client: -> tunnel_request")
	_ = control.SetReadDeadline(time.Now().Add(10 * time.Second))
	m, err = proto.Read(control)
	_ = control.SetReadDeadline(time.Time{})
	if err != nil {
		return fmt.Errorf("tunnel_ok 10s ichida kelmadi: %w", err)
	}
	log.Printf("client: <- %s", m.Type)
	if m.Type != "tunnel_ok" {
		return fmt.Errorf("%w: tunnel rad etildi: %s", errRejected, m.Reason)
	}
	if request.ID == "" {
		request.ID = m.TunnelID
	}
	if request.Kind == "http" && request.Domain == "" && request.Subdomain == "" {
		request.Subdomain = m.TunnelID
	}
	agent.Add(m.TunnelID, request.Local)
	fmt.Println(m.Domain)
	agentDone := make(chan error, 1)
	go func() { agentDone <- agent.Accept(s) }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-agentDone:
		if err == nil {
			return errors.New("yamux sessiyasi yopildi")
		}
		return err
	}
}

// RunCommand authenticates then sends one control-plane command, used for
// account-scoped operations such as custom domain management.
func (c *Client) RunCommand(ctx context.Context, command proto.Message) (proto.Message, error) {
	conn, err := c.dialControl(ctx)
	if err != nil {
		return proto.Message{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Time{})
	session, err := yamux.Client(conn, newYamuxConfig())
	if err != nil {
		return proto.Message{}, fmt.Errorf("yamux session: %w", err)
	}
	defer session.Close()
	control, err := session.OpenStream()
	if err != nil {
		return proto.Message{}, fmt.Errorf("control stream ochish: %w", err)
	}
	defer control.Close()
	if err := proto.Write(control, proto.Auth(c.Config.Token, c.Config.Version, runtime.GOOS)); err != nil {
		return proto.Message{}, fmt.Errorf("auth yuborish: %w", err)
	}
	_ = control.SetReadDeadline(time.Now().Add(10 * time.Second))
	response, err := proto.Read(control)
	_ = control.SetReadDeadline(time.Time{})
	if err != nil {
		return proto.Message{}, fmt.Errorf("auth_ok kutish: %w", err)
	}
	if response.Type != "auth_ok" {
		return proto.Message{}, fmt.Errorf("server rad etdi: %s", response.Reason)
	}
	if err := proto.Write(control, command); err != nil {
		return proto.Message{}, fmt.Errorf("buyruq yuborish: %w", err)
	}
	_ = control.SetReadDeadline(time.Now().Add(10 * time.Second))
	response, err = proto.Read(control)
	_ = control.SetReadDeadline(time.Time{})
	if err != nil {
		return proto.Message{}, fmt.Errorf("javob kutish: %w", err)
	}
	if response.Type == "domain_err" || response.Type == "tunnel_err" {
		return proto.Message{}, errors.New(response.Reason)
	}
	return response, nil
}

func (c *Client) dialControl(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	host, _, err := net.SplitHostPort(c.Config.Server)
	if err != nil {
		return nil, fmt.Errorf("server manzili noto'g'ri: %w", err)
	}
	if c.Config.Plain {
		if ip := net.ParseIP(host); !ip.IsLoopback() {
			return nil, errors.New("--plain faqat 127.0.0.1 yoki ::1 uchun ruxsat")
		}
		return dialer.DialContext(ctx, "tcp", c.Config.Server)
	}
	tlsConfig := c.Config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS13, ServerName: host}
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", c.Config.Server, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("TLS handshake: %w", err)
	}
	return conn, nil
}

func (c *Client) Run(ctx context.Context) error {
	if c.Config.Server == "" || c.Config.Token == "" {
		return errors.New("server va token berilishi kerak")
	}
	delay := time.Second
	for {
		err := c.connect(ctx)
		if ctx.Err() != nil {
			return nil
		}
		_ = err
		jitter := time.Duration(rand.Int64N(int64(delay/3 + 1)))
		wait := delay + jitter
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
}

func (c *Client) connect(ctx context.Context) error {
	conn, err := c.dialControl(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Time{})
	s, err := yamux.Client(conn, newYamuxConfig())
	if err != nil {
		return err
	}
	defer s.Close()
	control, err := s.OpenStream()
	if err != nil {
		return err
	}
	defer control.Close()
	if err := proto.Write(control, proto.Auth(c.Config.Token, c.Config.Version, runtime.GOOS)); err != nil {
		return err
	}
	response, err := proto.Read(control)
	if err != nil {
		return err
	}
	if response.Type != "auth_ok" {
		return fmt.Errorf("server rad etdi: %s", response.Reason)
	}
	c.mu.Lock()
	c.session = s
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.session = nil; c.mu.Unlock() }()
	if c.OnConnected != nil {
		if err := c.OnConnected(s); err != nil {
			return err
		}
	}
	for {
		_ = control.SetReadDeadline(time.Now().Add(45 * time.Second))
		msg, err := proto.Read(control)
		_ = control.SetReadDeadline(time.Time{})
		if err != nil {
			return err
		}
		if msg.Type == "ping" {
			if err := proto.Write(control, proto.Message{Type: "pong"}); err != nil {
				return err
			}
		}
	}
}
