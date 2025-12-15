package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	CTI      CTIConfig      `mapstructure:"cti"`
	Asterisk AsteriskConfig `mapstructure:"asterisk"`
	Service  ServiceConfig  `mapstructure:"service"`
	HTTP     HTTPConfig     `mapstructure:"http"`
}

type CTIConfig struct {
	WSURL string    `mapstructure:"ws_url"`
	Auth  AuthConfig `mapstructure:"auth"`
}

type AuthConfig struct {
	KeycloakURL  string `mapstructure:"keycloak_url"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
}

type AsteriskConfig struct {
	ARIURL           string `mapstructure:"ari_url"`
	Username         string `mapstructure:"username"`
	Password         string `mapstructure:"password"`
	OutboundEndpoint string `mapstructure:"outbound_endpoint"`
}

type ServiceConfig struct {
	LogLevel string `mapstructure:"log_level"`
}

type HTTPConfig struct {
	Port    int  `mapstructure:"port"`
	Enabled bool `mapstructure:"enabled"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Set defaults
	viper.SetDefault("cti.ws_url", "ws://10.224.111.206:8089/ws/ts")
	viper.SetDefault("asterisk.ari_url", "http://localhost:8088/ari")
	viper.SetDefault("asterisk.username", "ari")
	viper.SetDefault("asterisk.password", "ari")
	viper.SetDefault("asterisk.outbound_endpoint", "bank-out")
	viper.SetDefault("service.log_level", "info")
	viper.SetDefault("http.port", 8080)
	viper.SetDefault("http.enabled", true)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
