package main

import (
	"context"
	"log"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type Nostr struct {
	Relays []*nostr.Relay
}

func NewNostr(relays []string) Nostr {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := []*nostr.Relay{}
	for _, relay_url := range relays {
		r, err := nostr.RelayConnect(ctx, relay_url)
		if err != nil {
			log.Panic(err.Error())
		}
		result = append(result, r)
	}

	return Nostr{Relays: result}
}

func (n *Nostr) SingleQuery(filter nostr.Filter) []nostr.Event {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r := n.Relays[0]
	return r.QuerySync(ctx, filter)
}

func (n *Nostr) PublishEvent(ev nostr.Event) nostr.Status {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r := n.Relays[0]
	return r.Publish(ctx, ev)
}
