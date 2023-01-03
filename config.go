package main

import (
	"github.com/mitchellh/go-homedir"
)

var config Config

type Config struct {
	Active  string        `json:"active"`
	DataDir string        `json:"-"`
	Relays  []string      `json:"relays"`
	BTCRPC  *BTCRPCClient `json:"btcrpc"`
}

type BTCRPCClient struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func (c *Config) Init() {
	if c.Relays == nil {
		c.Relays = []string{}
	}
	if c.DataDir == "" {
		base_dir_exp, _ := homedir.Expand(BASE_DIR)
		c.DataDir = base_dir_exp
	}
}
