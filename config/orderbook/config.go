package config

import "go.uber.org/zap"

type Config struct {
	Coinbase  Coinbase
	ZapConfig zap.Config `yaml:"zap"`
	Sentry    Sentry     `yaml:"sentry"`
}

type Sentry struct {
	Dsn string
}

type Coinbase struct {
	CB_Secret     string
	CB_Key        string
	CB_Passphrase string
	CB_REST_API   string
	CB_WS         string
}

var AppConfig Config
