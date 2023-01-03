package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/nbd-wtf/go-nostr"
)

var kindNames = map[int]string{
	nostr.KindSetMetadata:            "Profile Metadata",
	nostr.KindTextNote:               "Text Note",
	nostr.KindRecommendServer:        "Relay Recommendation",
	nostr.KindContactList:            "Contact List",
	nostr.KindEncryptedDirectMessage: "Encrypted Message",
	nostr.KindDeletion:               "Deletion Notice",
}

func findEvent(db *LocalDB, n Nostr, id string) *nostr.Event {
	// pool := n.ReadPool()

	// _, all := pool.Sub(nostr.Filters{{IDs: []string{id}}})
	fmt.Printf("\nSearching event id: %s", id)

	for _, event := range n.SingleQuery(nostr.Filter{IDs: []string{id}}) {
		// for event := range nostr.Unique(all) {
		fmt.Printf("\nFound event id: %s", id)
		if event.ID != id {
			log.Printf("got unexpected event %s.\n", event.ID)
			continue
		}
		return &event
	}
	return nil
}

// Add event to my stream. In case there were no previous events on the Stream, make a genesis event.
func publishEvent(n Nostr, ev *nostr.Event) error {
	// pool := n.ReadPool()

	// TMP
	// ev2 := nostr.Event{
	// 	ID:        ev.ID,
	// 	PubKey:    ev.PubKey,
	// 	CreatedAt: ev.CreatedAt,
	// 	Kind:      ev.Kind,
	// 	Tags:      ev.Tags,
	// 	Content:   ev.Content,
	// 	Sig:       ev.Sig,
	// }
	fmt.Println(ev.GetExtra("ots"))
	// FIX: the problem is likely here https://github.com/nbd-wtf/go-nostr/blob/master/event_aux.go#L72-L74

	status := n.PublishEvent(*ev)
	if status != 1 {
		return fmt.Errorf("error publishing event. Status: %s", status)
	}

	return nil
}

func publishStream(n Nostr, es EventStream) error {
	last_published_id := "/"
	for _, ev := range es.Log {
		err := publishEvent(n, &ev)
		if err != nil {
			return fmt.Errorf("didn't manage to publish the event stream. Last published event ID: %s", last_published_id)
		}
		last_published_id = ev.ID
		// Don't spam
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// Find the next event in the hashchain
func findNextEvents(n Nostr, pubkey string, prev string) ([]*nostr.Event, error) {
	fmt.Printf("\nSearching for an event with pubkey: %s and prev: %s\n", pubkey, prev)
	// pool := n.ReadPool()
	// TODO: Smarter filter (created_at filter?)
	// _, events := r.Subscribe(context.Background(), nostr.Filters{{Authors: []string{pubkey}}})

	result := []*nostr.Event{}
	// Mapping from prev value to event struct. Used to construct the sequence that we return
	prev_to_event := map[string]nostr.Event{}

	for _, event := range n.SingleQuery(nostr.Filter{Authors: []string{pubkey}}) {
		for _, tag := range event.Tags {
			val, exists := prev_to_event[tag.Value()]
			if exists {
				fmt.Printf("\nConflict detected. Two events with the same prev. Ids: %s, %s", val.ID, event.ID)
				return nil, errors.New("Conflict")
			}
			prev_to_event[tag.Value()] = event
		}
	}

	// Construct a chain of events
	for {
		ev, ok := prev_to_event[prev]
		if !ok {
			return result, nil
		}
		result = append(result, &ev)
		prev = ev.ID
	}

	return result, nil

	// fix this mess
	// events_chan := nostr.Unique(events)
	// out := false

	// go func() {
	// 	time.Sleep(2 * time.Second)
	// 	out = true
	// }()

	// for {
	// 	if out {
	// 		// Construct a chain of events
	// 		for {
	// 			ev, ok := prev_to_event[prev]
	// 			if !ok {
	// 				return result, nil
	// 			}
	// 			result = append(result, &ev)
	// 			prev = ev.ID
	// 		}
	// 	}
	// 	select {
	// 	case event := <-events_chan:
	// 		for _, tag := range event.Tags {
	// 			if tag.Key() == "prev" {
	// 				fmt.Printf("\nGot event, ots: %s", event.GetExtraString("ots"))
	// 				ok, err := event.CheckSignature()
	// 				// both 'ok' needs to be true and err nil for a valid sig
	// 				if ok && err == nil {
	// 					val, exists := prev_to_event[tag.Value()]
	// 					if exists {
	// 						fmt.Printf("\nConflict detected. Two events with the same prev. Ids: %s, %s", val.ID, event.ID)
	// 						return nil, errors.New("Conflict")
	// 					}
	// 					prev_to_event[tag.Value()] = event
	// 				} else {
	// 					// We ignore events with invalid signature
	// 					fmt.Printf("\nFound an event with an invalid signature. Event id: %s", event.ID)
	// 				}
	// 			}
	// 		}
	// 	default:
	// 	}
	// }
}

func printEvent(evt nostr.Event, name *string, verbose bool) {
	kind, ok := kindNames[evt.Kind]
	if !ok {
		kind = "Unknown Kind"
	}

	var ID string = shorten(evt.ID)
	var fromField string = shorten(evt.PubKey)
	var prev string = get_prev(evt)

	if name != nil {
		fromField = fmt.Sprintf("%s (%s)", *name, shorten(evt.PubKey))
	}

	if verbose {
		ID = evt.ID

		if name == nil {
			fromField = evt.PubKey
		} else {
			fromField = fmt.Sprintf("%s (%s)", *name, evt.PubKey)
		}
	}

	fmt.Printf("Id: %s\n", ID)
	fmt.Printf("Prev: %s\n", prev)
	fmt.Printf("From: %s\n", fromField)
	fmt.Printf("Time: %s\n", humanize.Time(evt.CreatedAt))
	fmt.Printf("Type: %s\n", kind)
	fmt.Printf("\n")

	switch evt.Kind {
	// TODO: Support other kinds
	case nostr.KindTextNote:
		fmt.Print("  " + strings.ReplaceAll(evt.Content, "\n", "\n  "))
	default:
		fmt.Print(evt.Content)
	}
}

func get_prev(evt nostr.Event) string {
	for _, tag := range evt.Tags {
		if tag.Key() == "prev" {
			return tag.Value()
		}
	}
	return "Not set"
}

func shorten(id string) string {
	if len(id) < 12 {
		return id
	}
	return id[0:4] + "..." + id[len(id)-4:]
}
