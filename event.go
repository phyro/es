package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

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

func findEvent(n *Nostr, id string) (*nostr.Event, error) {
	fmt.Printf("\nSearching event id: %s", id)
	evs, err := n.SingleQueryPool(nostr.Filter{IDs: []string{id}})
	if err != nil {
		return nil, err
	}
	for _, event := range evs {
		if event.ID != id {
			log.Printf("got unexpected event %s.\n", event.ID)
			continue
		}
		return &event, nil
	}

	return nil, errors.New("event not found")
}

// Add event to my stream. In case there were no previous events on the Stream, make a genesis event.
// func publishEvent(n *Nostr, ev *nostr.Event) error {
// 	status, err := n.PublishEvent(*ev)
// 	if err != nil {
// 		return err
// 	}
// 	if status != 1 {
// 		return fmt.Errorf("error publishing event. Status: %s", status)
// 	}

// 	return nil
// }

// func publishStream(n *Nostr, es *EventStream) error {
// 	last_published_id := "/"
// 	for _, ev := range es.Log {
// 		err := publishEvent(n, &ev)
// 		if err != nil {
// 			return fmt.Errorf("didn't manage to publish the event stream. Last published event ID: %s", last_published_id)
// 		}
// 		last_published_id = ev.ID
// 		// Don't spam
// 		time.Sleep(100 * time.Millisecond)
// 	}

// 	return nil
// }

// Find the next events in the hashchain
func findNextEvents(n *Nostr, pubkey string, prev string) ([]*nostr.Event, error) {
	result := []*nostr.Event{}
	// Mapping from prev value to event struct. Used to construct the sequence that we return
	prev_to_event := map[string]nostr.Event{}
	evs, err := n.SingleQueryPool(nostr.Filter{Authors: []string{pubkey}})
	if err != nil {
		return nil, err
	}
	// TODO: make the evs slice unique
	for _, ev := range evs {
		for _, tag := range ev.Tags {
			if tag.Key() != "prev" {
				continue
			}
			entry, exists := prev_to_event[tag.Value()]
			// if the entry exists, make sure the entry has the same id as event id
			if exists && ev.ID != entry.ID {
				fmt.Printf("\nConflict detected. Two events with the same prev. Ids: %s, %s", entry.ID, ev.ID)
				return nil, errors.New("Conflict")
			} else {
				prev_to_event[tag.Value()] = ev
			}
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
	fmt.Printf("Author: %s\n", fromField)
	fmt.Printf("Date: %s (✓)\n", humanize.Time(evt.CreatedAt))
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
