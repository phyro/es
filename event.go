package main

import (
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

func showEvent(db *LocalDB, n Nostr, id string, verbose bool) {
	pool := n.ReadPool()

	_, all := pool.Sub(nostr.Filters{{IDs: []string{id}}})
	for event := range nostr.Unique(all) {
		if event.ID != id {
			log.Printf("got unexpected event %s.\n", event.ID)
			continue
		}

		// Check if we have a name for the event stream owner
		var name *string
		es, err := db.GetEventStream(event.PubKey)
		if err == nil {
			name = &es.Name
		}
		printEvent(event, name, verbose)
		break
	}
}

// Add event to my stream. In case there were no previous events on the Stream, make a genesis event.
func publishEvent(n Nostr, priv_key string, content string, prev string) (*nostr.Event, error) {
	pool := n.WritePool(priv_key)

	// var tags nostr.Tags
	// tags = append(tags, nostr.Tag{"prev", prev})
	tags := nostr.Tags{nostr.Tag{"prev", prev}}

	event, statuses, err := pool.PublishEvent(&nostr.Event{
		CreatedAt: time.Now(),
		Kind:      nostr.KindTextNote,
		Tags:      tags,
		Content:   content,
	})
	if err != nil {
		log.Printf("Error publishing: %s.\n", err.Error())
		return nil, err
	}

	printPublishStatus(event, statuses, len(n.Relays))
	return event, nil
}

func printPublishStatus(event *nostr.Event, statuses chan nostr.PublishStatus, remaining int) {
	for status := range statuses {
		switch status.Status {
		case nostr.PublishStatusSent:
			fmt.Printf("Sent event %s to '%s'.\n", event.ID, status.Relay)
		case nostr.PublishStatusFailed:
			fmt.Printf("Failed to send event %s to '%s'.\n", event.ID, status.Relay)
		case nostr.PublishStatusSucceeded:
			fmt.Printf("Seen %s on '%s'.\n", event.ID, status.Relay)
		}
		remaining -= 1
		if remaining == 0 {
			return
		}
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

	fmt.Printf("\n")
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
