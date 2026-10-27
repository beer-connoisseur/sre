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
	MetricsPort     string        `env:"METRICS_PORT" envDefault:"9090"`
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

	Chaos struct {
		ErrorRate float64       `env:"CHAOS_ERROR_RATE" envDefault:"0"`
		Latency   time.Duration `env:"CHAOS_LATENCY" envDefault:"0s"`
	}
}

type Prober struct {
	TargetURL       string        `env:"PROBER_TARGET_URL,required,notEmpty"`
	Interval        time.Duration `env:"PROBER_INTERVAL" envDefault:"10s"`
	Timeout         time.Duration `env:"PROBER_TIMEOUT" envDefault:"3s"`
	MetricsPort     string        `env:"METRICS_PORT" envDefault:"9090"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`
	LogLevel        string        `env:"LOG_LEVEL" envDefault:"info"`
}

func New() (*Config, error) {
	return parse[Config]()
}

func NewProber() (*Prober, error) {
	return parse[Prober]()
}

func (c *Config) Level() string {
	return c.LogLevel
}

func (c *Prober) Level() string {
	return c.LogLevel
}

func parse[T any]() (*T, error) {
	var cfg T
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
