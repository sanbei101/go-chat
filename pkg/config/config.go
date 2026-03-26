package config

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "config.yaml"

// Config 是整个项目的统一配置入口。
type Config struct {
	Gateway  GatewayConfig  `yaml:"gateway"`
	Postgres PostgresConfig `yaml:"postgres"`
	Redis    RedisConfig    `yaml:"redis"`
	Worker   WorkerConfig   `yaml:"worker"`
}

type GatewayConfig struct {
	Addr              string        `yaml:"addr"`
	GRPCAddr          string        `yaml:"grpc_addr"`
	Path              string        `yaml:"path"`
	HandshakeTimeout  time.Duration `yaml:"handshake_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	SendQueueSize     int           `yaml:"send_queue_size"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

type WorkerConfig struct {
	GRPCAddr string `yaml:"grpc_addr"`
}

// Load 从 yaml 文件加载配置，并补齐默认值。
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate 校验关键配置，避免服务启动后再暴露问题。
func (c *Config) Validate() error {
	if c.Gateway.Addr == "" {
		return errors.New("config: gateway.addr is required")
	}
	if c.Gateway.GRPCAddr == "" {
		return errors.New("config: gateway.grpc_addr is required")
	}
	if c.Worker.GRPCAddr == "" {
		return errors.New("config: worker.grpc_addr is required")
	}
	return nil
}

func defaultConfig() *Config {
	return &Config{
		Gateway: GatewayConfig{
			Addr:              ":8080",
			GRPCAddr:          ":9090",
			Path:              "/ws",
			HandshakeTimeout:  10 * time.Second,
			WriteTimeout:      5 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			ShutdownTimeout:   10 * time.Second,
			SendQueueSize:     64,
		},
		Redis: RedisConfig{
			Addr: "127.0.0.1:6379",
		},
		Worker: WorkerConfig{
			GRPCAddr: ":9091",
		},
	}
}
