package config

import (
	"net"
	"net/url"
	"time"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	Postgres Postgres
	HTTP     HTTP
	Kafka    Kafka
	Topics   Topics
	Outbox   Outbox
	JWT      JWT
	Docs     Docs
}

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

type Topics struct {
	Users string `env:"KAFKA_TOPIC_USERS" envDefault:"users"`
}

type Outbox struct {
	Interval  time.Duration `env:"OUTBOX_INTERVAL" envDefault:"1s"`
	BatchSize int           `env:"OUTBOX_BATCH_SIZE" envDefault:"100"`
}

type JWT struct {
	Secret string `env:"JWT_SECRET,required"`
	Issuer string `env:"JWT_ISSUER" envDefault:"my-unique-issuer-key"`
}

type Docs struct {
	Username string `env:"DOCS_USERNAME" envDefault:"admin"`
	Password string `env:"DOCS_PASSWORD" envDefault:"admin"`
}

func New() (*Config, error) {
	cfg := new(Config)
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
