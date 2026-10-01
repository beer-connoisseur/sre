package config

import (
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	HTTPPort        string        `env:"HTTP_PORT" envDefault:"8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
	LogLevel        string        `env:"LOG_LEVEL" envDefault:"info"`

	PG struct {
		Host     string `env:"POSTGRES_HOST" envDefault:"localhost"`
		Port     string `env:"POSTGRES_PORT" envDefault:"5432"`
		DB       string `env:"POSTGRES_DB" envDefault:"shortener"`
		User     string `env:"POSTGRES_USER,required,notEmpty"`
		Password string `env:"POSTGRES_PASSWORD,required,notEmpty"`
		SSLMode  string `env:"POSTGRES_SSLMODE" envDefault:"disable"`
		MaxConns int    `env:"POSTGRES_MAX_CONNS" envDefault:"10"`
	}
}

func New() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) PostgresURL() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.PG.User, c.PG.Password),
		Host:   net.JoinHostPort(c.PG.Host, c.PG.Port),
		Path:   c.PG.DB,
	}

	q := u.Query()
	q.Set("sslmode", c.PG.SSLMode)
	q.Set("pool_max_conns", strconv.Itoa(c.PG.MaxConns))
	u.RawQuery = q.Encode()

	return u.String()
}
