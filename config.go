package main

import (
	"github.com/mitchellh/go-homedir"
)

var config Config

type Config struct {
	Active  string            `json:"active"`
	DataDir string            `json:"-"`
	Relays  map[string]Policy `json:"relays,flow"`
	BTCRPC  BTCRPCClient      `json:"btcrpc"`
}

type BTCRPCClient struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type Policy struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
}

func (p Policy) String() string {
	var ret string
	if p.Read {
		ret += "r"
	}
	if p.Write {
		ret += "w"
	}
	return ret
}

func (c *Config) Init() {
	if c.Relays == nil {
		c.Relays = make(map[string]Policy)
	}
	if c.DataDir == "" {
		base_dir_exp, _ := homedir.Expand(BASE_DIR)
		c.DataDir = base_dir_exp
	}
}
