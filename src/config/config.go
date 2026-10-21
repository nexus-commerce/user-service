package config

import (
	"net"
	"net/url"
	"time"

	"github.com/caarlos0/env/v6"
)

type Postgres struct {
	Host     string `env:"POSTGRES_HOST,required"`
	Port     string `env:"POSTGRES_PORT" envDefault:"5432"`
	Database string `env:"POSTGRES_DATABASE,required"`
	User     string `env:"POSTGRES_USER,required"`
	Password string `env:"POSTGRES_PASSWORD,required"`
	SSLMode  string `env:"POSTGRES_SSLMODE" envDefault:"disable"`
}

func (p Postgres) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(p.User, p.Password),
		Host:     net.JoinHostPort(p.Host, p.Port),
		Path:     p.Database,
		RawQuery: url.Values{"sslmode": {p.SSLMode}}.Encode(),
	}
	return u.String()
}

type HTTP struct {
	Port string `env:"HTTP_PORT" envDefault:"80"`
}

type Kafka struct {
	Host string `env:"KAFKA_HOST,required"`
	Port string `env:"KAFKA_PORT" envDefault:"29092"`
}

func (k *Kafka) Addr() string {
	return net.JoinHostPort(k.Host, k.Port)
}

type Outbox struct {
	Interval  time.Duration `env:"OUTBOX_INTERVAL" envDefault:"1s"`
	BatchSize int           `env:"OUTBOX_BATCH_SIZE" envDefault:"100"`
}

type Purge struct {
	Interval time.Duration `env:"PURGE_INTERVAL" envDefault:"1h"`
	Grace    time.Duration `env:"PURGE_GRACE" envDefault:"168h"`
}

type JWT struct {
	Secret string `env:"JWT_SECRET,required"`
	Issuer string `env:"JWT_ISSUER" envDefault:"my-unique-issuer-key"`
}

type Docs struct {
	Username string `env:"DOCS_USERNAME" envDefault:"admin"`
	Password string `env:"DOCS_PASSWORD" envDefault:"admin"`
}

type Notification struct {
	PublicKey string `env:"NOTIFICATION_PUBLIC_KEY"`
	KeyID     string `env:"NOTIFICATION_KEY_ID"`
}

func Load[T any]() (*T, error) {
	cfg := new(T)
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
