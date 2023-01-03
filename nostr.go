package main

import (
	"context"
	"log"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type Nostr struct {
	Relays []nostr.Relay
}

func NewNostr(relays []string) Nostr {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := []nostr.Relay{}
	for _, relay_url := range relays {
		r, err := nostr.RelayConnect(ctx, relay_url)
		if err != nil {
			log.Panic(err.Error())
		}
		result = append(result, *r)
	}
	return Nostr{Relays: result}
}

// 	// pool := nostr.NewRelayPool()

// 	// for relay, policy := range n.Relays {
// 	// 	cherr := pool.Add(relay, nostr.SimplePolicy{
// 	// 		Read:  policy.Read,
// 	// 		Write: policy.Write,
// 	// 	})
// 	// 	err := <-cherr
// 	// 	if err != nil {
// 	// 		log.Printf("error adding relay '%s': %s", relay, err.Error())
// 	// 	}
// 	// }

// 	// hasRelays := false
// 	// pool.Relays.Range(func(_ string, _ *nostr.Relay) bool {
// 	// 	hasRelays = true
// 	// 	return false
// 	// })
// 	// if !hasRelays {
// 	// 	log.Printf("You have zero relays configured, everything will probably fail.")
// 	// }

// 	// go func() {
// 	// 	for notice := range pool.Notices {
// 	// 		log.Printf("%s has sent a notice: '%s'\n", notice.Relay, notice.Message)
// 	// 	}
// 	// }()

// 	// return pool
// }

// func (n *Nostr) WritePool(priv_key string) *nostr.RelayPool {
// 	// pool := n.Init()
// 	// pool.SecretKey = &priv_key
// 	// return pool
// }

// func (n *Nostr) RelayConnect() *nostr.Relay {
// 	// return n.Init()
// 	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
// 	defer cancel()
// 	// TODO: make relay pooling
// 	keys := make([]string, 0, len(n.Relays))
// 	for k := range n.Relays {
// 		keys = append(keys, k)
// 	}
// 	relay_url := keys[0]
// 	r, err := nostr.RelayConnect(ctx, relay_url)
// 	if err != nil {
// 		t.Fatalf("RelayConnectContext: %v", err)
// 	}
// }

func (n *Nostr) SingleQuery(filter nostr.Filter) []nostr.Event {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r := n.Relays[0]
	return r.QuerySync(ctx, filter)
}

func (n *Nostr) PublishEvent(ev nostr.Event) nostr.Status {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r := n.Relays[0]
	return r.Publish(ctx, ev)
}
