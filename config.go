package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

const CONFIG_BASE_DIR = "~/.config/nostr"
const CONFIG_FILE = "config.json"

type Config struct {
	DataDir string        `json:"-"`
	Relays  []string      `json:"relays"`
	BTCRPC  *BTCRPCClient `json:"btcrpc"`
}

func (c *Config) Init() {
	if c.Relays == nil {
		c.Relays = []string{}
	}
	if c.DataDir == "" {
		base_dir_exp, _ := homedir.Expand(CONFIG_BASE_DIR)
		c.DataDir = base_dir_exp
	}
}

func (c *Config) Load() {
	// Make config folder
	base_dir_exp, _ := homedir.Expand(CONFIG_BASE_DIR)
	os.Mkdir(base_dir_exp, 0700)
	path := filepath.Join(base_dir_exp, CONFIG_FILE)
	_, err := os.Open(path)
	if err != nil {
		// File doesn't exist, create it
		c = &Config{DataDir: base_dir_exp}
		c.Save()
		_, _ = os.Open(path)
	}
	f, _ := os.Open(path)
	err = json.NewDecoder(f).Decode(c)
	if err != nil {
		log.Fatal("can't parse config file " + path + ": " + err.Error())
	}
	c.Init()
}

func (c *Config) Save() {
	path := filepath.Join(c.DataDir, CONFIG_FILE)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal("can't open config file " + path + ": " + err.Error())
		return
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(*c)
}

// Implement BitcoinRPCManager interface
func (c *Config) GetBitcoinRPC() *BTCRPCClient {
	return c.BTCRPC
}

func (c *Config) ConfigureBitcoinRPC(host string, user string, password string) error {
	c.BTCRPC = &BTCRPCClient{
		Host:     host,
		User:     user,
		Password: password,
	}
	client, err := newBtcConn(host, user, password)
	if err != nil {
		return err
	}
	ver, err := client.BackendVersion()
	if err != nil {
		return err
	}
	fmt.Printf("Bitcoin node version: %d\n", ver)
	c.Save()

	return nil
}

func (c *Config) UnsetBitcoinRPC() {
	c.BTCRPC = nil
	c.Save()
}

// Implement Relayer interface
func (c *Config) AddRelay(url string) {
	c.Relays = append(c.Relays, url)
	fmt.Printf("Added relay %s.\n", url)
	c.Save()
}

func (c *Config) RemoveRelay(url string) {
	result := []string{}
	found := false
	for _, relay_url := range c.Relays {
		if relay_url != url {
			result = append(result, relay_url)
		} else {
			found = true
		}
	}
	if !found {
		fmt.Printf("Could not find relay %s\n", url)
	} else {
		fmt.Printf("Removed relay %s.\n", url)
		c.Relays = result
		c.Save()
	}
}

func (c *Config) ListRelays() []string {
	return c.Relays
}
