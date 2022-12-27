package main

import (
	"fmt"
	"log"

	"github.com/nbd-wtf/go-nostr"
)

func world(db *LocalDB, n Nostr, event_streams []EventStream, verbose bool) {
	if len(event_streams) == 0 {
		log.Println("You need to be following at least one stream to run 'world'")
		return
	}
	// Before listening, we have to sync all event streams to their HEAD
	sync_all(n, event_streams)

	pool := n.ReadPool()

	var keys []string
	for _, es := range event_streams {
		keys = append(keys, es.PubKey)
	}

	_, all := pool.Sub(nostr.Filters{{Authors: keys}})
	for event := range nostr.Unique(all) {
		handle_event(db, event)
	}
}

func sync_all(n Nostr, ess []EventStream) {
	fmt.Println("Syncing event streams. This may take a while...")
	for _, es := range ess {
		es.Sync(n)
		db.SaveEventStream(es)
	}
	fmt.Println("\nEvent streams synced.")
}

func handle_event(db *LocalDB, ev nostr.Event) {
	// Find the expected head of the event stream
	es, err := db.GetEventStream(ev.PubKey)
	if err != nil {
		log.Panic(err.Error())
	}
	expected_prev := es.GetHead()
	prev := get_prev(ev)
	if prev != expected_prev {
		fmt.Printf("\nIgnoring event %s from %s.", ev.ID, es.Name)
		fmt.Printf("\nExpected prev %s got %s.", expected_prev, prev)
		fmt.Println()
	}

	// Append event to the event stream chain
	es.Append(ev) // validates the event including pubkey and sig
	db.SaveEventStream(es)

	printEvent(ev, &es.Name, true)
}
