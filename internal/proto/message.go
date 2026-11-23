// Package proto contains the versioned control and data-stream wire protocol.
package proto

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const Version = 1
const maxMessageSize = 1 << 20

type Message struct {
	Type              string         `json:"type"`
	Version           int            `json:"version,omitempty"`
	Token             string         `json:"token,omitempty"`
	ClientVersion     string         `json:"client_version,omitempty"`
	OS                string         `json:"os,omitempty"`
	ClientID          string         `json:"client_id,omitempty"`
	Reason            string         `json:"reason,omitempty"`
	TunnelID          string         `json:"tunnel_id,omitempty"`
	ID                string         `json:"id,omitempty"`
	Kind              string         `json:"kind,omitempty"`
	Local             string         `json:"local,omitempty"`
	Subdomain         string         `json:"subdomain,omitempty"`
	Domain            string         `json:"domain,omitempty"`
	RemotePort        int            `json:"remote_port,omitempty"`
	RemoteAddr        string         `json:"remote_addr,omitempty"`
	Host              string         `json:"host,omitempty"`
	Options           map[string]any `json:"options,omitempty"`
	VerificationToken string         `json:"verification_token,omitempty"`
	Verified          bool           `json:"verified,omitempty"`
	Domains           []DomainInfo   `json:"domains,omitempty"`
}

type DomainInfo struct {
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

func Auth(token, clientVersion, os string) Message {
	return Message{Type: "auth", Version: Version, Token: token, ClientVersion: clientVersion, OS: os}
}

// Write serializes one framed JSON message. A frame is a uint32 big-endian size followed by JSON.
func Write(w io.Writer, m Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	if len(b) > maxMessageSize {
		return fmt.Errorf("message too large")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(b)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// Read receives one framed JSON message.
func Read(r io.Reader) (Message, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Message{}, err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 || n > maxMessageSize {
		return Message{}, fmt.Errorf("invalid message length: %d", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return Message{}, err
	}
	var m Message
	if err := json.Unmarshal(b, &m); err != nil {
		return Message{}, fmt.Errorf("decode message: %w", err)
	}
	return m, nil
}
