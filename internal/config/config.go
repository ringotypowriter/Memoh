package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

const (
	DefaultConfigPath       = "config.toml"
	DefaultHTTPAddr         = ":8080"
	DefaultNamespace        = "default"
	DefaultSocketPath       = "/run/containerd/containerd.sock"
	DefaultBusyboxImg       = "docker.io/library/busybox:latest"
	DefaultDataRoot         = "data"
	DefaultDataMount        = "/data"
	DefaultJWTExpiresIn     = "24h"
	DefaultPGHost           = "127.0.0.1"
	DefaultPGPort           = 5432
	DefaultPGUser           = "postgres"
	DefaultPGDatabase       = "memoh"
	DefaultPGSSLMode        = "disable"
	DefaultQdrantURL        = "http://127.0.0.1:6334"
	DefaultQdrantCollection = "memory"
)

type Config struct {
	Server     ServerConfig     `toml:"server"`
	Auth       AuthConfig       `toml:"auth"`
	Containerd ContainerdConfig `toml:"containerd"`
	MCP        MCPConfig        `toml:"mcp"`
	Postgres   PostgresConfig   `toml:"postgres"`
	Qdrant     QdrantConfig     `toml:"qdrant"`
}

type ServerConfig struct {
	Addr string `toml:"addr"`
}

type AuthConfig struct {
	JWTSecret    string `toml:"jwt_secret"`
	JWTExpiresIn string `toml:"jwt_expires_in"`
}

type ContainerdConfig struct {
	SocketPath string `toml:"socket_path"`
	Namespace  string `toml:"namespace"`
}

type MCPConfig struct {
	BusyboxImage string `toml:"busybox_image"`
	Snapshotter  string `toml:"snapshotter"`
	DataRoot     string `toml:"data_root"`
	DataMount    string `toml:"data_mount"`
}

type PostgresConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	SSLMode  string `toml:"sslmode"`
}

type QdrantConfig struct {
	BaseURL        string `toml:"base_url"`
	APIKey         string `toml:"api_key"`
	Collection     string `toml:"collection"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
}

func Load(path string) (Config, error) {
	cfg := Config{
		Server: ServerConfig{
			Addr: DefaultHTTPAddr,
		},
		Auth: AuthConfig{
			JWTExpiresIn: DefaultJWTExpiresIn,
		},
		Containerd: ContainerdConfig{
			SocketPath: DefaultSocketPath,
			Namespace:  DefaultNamespace,
		},
		MCP: MCPConfig{
			BusyboxImage: DefaultBusyboxImg,
			DataRoot:     DefaultDataRoot,
			DataMount:    DefaultDataMount,
		},
		Postgres: PostgresConfig{
			Host:     DefaultPGHost,
			Port:     DefaultPGPort,
			User:     DefaultPGUser,
			Database: DefaultPGDatabase,
			SSLMode:  DefaultPGSSLMode,
		},
		Qdrant: QdrantConfig{
			BaseURL:    DefaultQdrantURL,
			Collection: DefaultQdrantCollection,
		},
	}

	if path == "" {
		path = DefaultConfigPath
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
