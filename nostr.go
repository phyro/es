package main

import (
	"log"

	"github.com/nbd-wtf/go-nostr"
)

type Nostr struct {
	Relays map[string]Policy
}

func (n *Nostr) Init() *nostr.RelayPool {
	pool := nostr.NewRelayPool()

	for relay, policy := range n.Relays {
		cherr := pool.Add(relay, nostr.SimplePolicy{
			Read:  policy.Read,
			Write: policy.Write,
		})
		err := <-cherr
		if err != nil {
			log.Printf("error adding relay '%s': %s", relay, err.Error())
		}
	}

	hasRelays := false
	pool.Relays.Range(func(_ string, _ *nostr.Relay) bool {
		hasRelays = true
		return false
	})
	if !hasRelays {
		log.Printf("You have zero relays configured, everything will probably fail.")
	}

	go func() {
		for notice := range pool.Notices {
			log.Printf("%s has sent a notice: '%s'\n", notice.Relay, notice.Message)
		}
	}()

	return pool
}

func (n *Nostr) WritePool(priv_key string) *nostr.RelayPool {
	pool := n.Init()
	pool.SecretKey = &priv_key
	return pool
}

func (n *Nostr) ReadPool() *nostr.RelayPool {
	return n.Init()
}
