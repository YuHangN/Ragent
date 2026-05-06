package config

import (
	"time"
)

type IngestionConfig struct {
	Tika   TikaConfig   `mapstructure:"tika"`
	Feishu FeishuConfig `mapstructure:"feishu"`
	HTTP   HTTPConfig   `mapstructure:"http"`
	Local  LocalConfig  `mapstructure:"local"`
}

type TikaConfig struct {
	URL            string `mapstructure:"url"`
	TimeoutSeconds int    `mapstructure:"timeout-seconds"`
}

func (c TikaConfig) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

type FeishuConfig struct {
	AppID     string `mapstructure:"app-id"`
	AppSecret string `mapstructure:"app-secret"`
}

type HTTPConfig struct {
	TimeoutSeconds int `mapstructure:"timeout-seconds"`
	MaxBodyMB      int `mapstructure:"max-body-mb"`
}

func (c HTTPConfig) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c HTTPConfig) MaxBodyBytes() int64 {
	if c.MaxBodyMB <= 0 {
		return 50 * 1024 * 1024
	}
	return int64(c.MaxBodyMB) * 1024 * 1024
}

type LocalConfig struct {
	AllowedRoots []string `mapstructure:"allowed-roots"`
}
