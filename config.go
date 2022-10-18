package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ApiId           int    `yaml:"api_id"`
	ApiHash         string `yaml:"api_hash"`
	BotToken        string `yaml:"bot_token"`
	BearerToken     string `yaml:"bearer_token"`
	WhitelistChatId int64  `yaml:"whitelist_chat_id"`
}

func loadConfig() (*Config, error) {
	var config Config
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
