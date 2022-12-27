package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

const GENESIS = "NULL"

type EventStream struct {
	Name    string        `json:"name"`
	PrivKey string        `json:"privkey"`
	PubKey  string        `json:"pubkey"`
	Log     []nostr.Event `json:"log,flow"`
	// TODO: list of relays the stream can be fetched from
}

func (es *EventStream) Append(ev nostr.Event) {
	// Check pubkey and verify signature
	if ev.PubKey != es.PubKey {
		log.Printf("Can't append event from pubkey %s to stream with pubkey %s.", ev.PubKey, es.PubKey)
		return
	}
	ok, err := ev.CheckSignature()
	if err != nil {
		log.Printf("ERROR :: Append :: Invalid signature for event: %s", ev.ID)
		log.Println(err.Error())
		return
	}
	if !ok {
		// According to code, if the signature is not valid 'ok' will be false
		log.Printf("ERROR :: Append :: Signature verification failed for event: %s", ev.ID)
		return
	}

	// Aal izz well, append event to the stream
	es.Log = append(es.Log, ev)
}

// Sync a stream - find the latest HEAD and query for the next event of the stream and repeat
func (es *EventStream) Sync(n Nostr) {
	fmt.Printf("Syncing %s ... ", es.Name)
	prev := es.GetHead()

	// Start from the genesis event and iterate forward
	for {
		pool := n.ReadPool()
		events, err := findNextEvents(pool, es.PubKey, prev)
		if err != nil {
			fmt.Println(err.Error())
			break
		}
		if len(events) == 0 {
			break
		}
		for _, ev := range events {
			es.Append(*ev)
			prev = ev.ID
		}
	}
	fmt.Printf("Done\nHEAD (%s) at: %s", es.Name, es.GetHead())
}

func (es *EventStream) Print(show_chain bool) {
	fmt.Printf("%s (%s)\n", es.Name, es.PubKey)
	if !show_chain {
		return
	}
	indent := "\t\t\t"
	fmt.Printf("\nEvent stream:\n")
	fmt.Printf("\n%s%s", indent, GENESIS)
	if es.Size() == 0 {
		return
	}

	fmt.Printf("\n%s|", indent)
	fmt.Printf("\n%sv", indent)
	for idx, event := range es.Log {
		fmt.Printf("\n%s", event.ID)
		if idx != es.Size()-1 {
			fmt.Printf("\n%s|", indent)
			fmt.Printf("\n%sv", indent)
		}
	}
}

func (es *EventStream) Size() int {
	return len(es.Log)
}

// Computes the HEAD from the log
func (es *EventStream) GetHead() string {
	prev_to_id := make(map[string]string)

	for _, event := range es.Log {
		for _, tag := range event.Tags {
			if tag.Key() == "prev" {
				// TODO: check if we already had this "prev"
				prev_to_id[tag.Value()] = event.ID
			}
		}
	}

	prev := GENESIS
	for {
		next, found := prev_to_id[prev]
		if !found {
			return prev
		} else {
			prev = next
		}
	}
}

// Find the next event in the hashchain (Verified sig check included)
func findNextEvents(pool *nostr.RelayPool, pubkey string, prev string) ([]*nostr.Event, error) {
	// TODO: Smarter filter (created_at filter?)
	_, events := pool.Sub(nostr.Filters{{Authors: []string{pubkey}}})

	// fix this mess
	events_chan := nostr.Unique(events)
	out := false

	go func() {
		time.Sleep(2 * time.Second)
		out = true
	}()

	result := []*nostr.Event{}
	prev_to_event := map[string]nostr.Event{}

	for {
		if out {
			// Construct a chain of events
			for {
				ev, ok := prev_to_event[prev]
				if !ok {
					return result, nil
				}
				result = append(result, &ev)
				prev = ev.ID
			}
		}
		select {
		case event := <-events_chan:
			for _, tag := range event.Tags {
				if tag.Key() == "prev" {
					ok, err := event.CheckSignature()
					// both 'ok' needs to be true and err nil for a valid sig
					if ok && err == nil {
						val, exists := prev_to_event[tag.Value()]
						if exists {
							fmt.Printf("\nConflict detected. Two events with the same prev. Ids: %s, %s", val.ID, event.ID)
							return nil, errors.New("Conflict")
						}
						prev_to_event[tag.Value()] = event
					} else {
						// We ignore events with invalid signature
						fmt.Printf("\nFound an event with an invalid signature. Event id: %s", event.ID)
					}
				}
			}
		default:
		}
	}
}
