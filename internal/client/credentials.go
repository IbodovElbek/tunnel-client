package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type TunnelFile struct {
	Server  string                `yaml:"server"`
	Token   string                `yaml:"token"`
	Tunnels map[string]TunnelSpec `yaml:"tunnels"`
}
type TunnelSpec struct {
	Kind       string `yaml:"kind"`
	Local      string `yaml:"local"`
	Subdomain  string `yaml:"subdomain"`
	Domain     string `yaml:"domain"`
	RemotePort int    `yaml:"remote_port"`
}

// LoadTunnelFile reads a multi-tunnel configuration and expands ${NAME}
// variables from the current environment without storing their secrets.
func LoadTunnelFile(path string) (TunnelFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return TunnelFile{}, fmt.Errorf("tunnel config o'qilmadi: %w", err)
	}
	var cfg TunnelFile
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(b))), &cfg); err != nil {
		return TunnelFile{}, fmt.Errorf("tunnel config noto'g'ri: %w", err)
	}
	if strings.TrimSpace(cfg.Server) == "" || strings.TrimSpace(cfg.Token) == "" || len(cfg.Tunnels) == 0 {
		return TunnelFile{}, fmt.Errorf("server, token va tunnels berilishi kerak")
	}
	for name, tunnel := range cfg.Tunnels {
		if tunnel.Kind != "http" && tunnel.Kind != "tcp" {
			return TunnelFile{}, fmt.Errorf("%s: kind http yoki tcp bo'lishi kerak", name)
		}
		if strings.TrimSpace(tunnel.Local) == "" {
			return TunnelFile{}, fmt.Errorf("%s: local berilishi kerak", name)
		}
	}
	return cfg, nil
}

type Credentials struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

func CredentialsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tunnel", "config.yml"), nil
}
func SaveCredentials(c Credentials) error {
	p, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err = os.WriteFile(p, b, 0600); err != nil {
		return err
	}
	return os.Chmod(p, 0600)
}
func LoadCredentials() (Credentials, error) {
	p, err := CredentialsPath()
	if err != nil {
		return Credentials{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Credentials{}, fmt.Errorf("login ma'lumotini o'qib bo'lmadi: %w", err)
	}
	var c Credentials
	err = yaml.Unmarshal(b, &c)
	return c, err
}
