package main

import (
	"fmt"
	"log"

	"github.com/nbd-wtf/go-nostr"
)

func world(db *LocalDB, n Nostr, event_streams []EventStream, verbose bool, rpcclient *BTCRPCClient) {
	if len(event_streams) == 0 {
		log.Println("You need to be following at least one stream to run 'world'")
		return
	}
	// Before listening, we have to sync all event streams to their HEAD
	sync_all(n, event_streams, rpcclient)
	var keys []string
	for _, es := range event_streams {
		keys = append(keys, es.PubKey)
	}

	// Find events for the streams we follow
	for _, event := range n.SingleQuery(nostr.Filter{Authors: keys}) {
		handle_event(db, event, rpcclient)
	}
}

func sync_all(n Nostr, ess []EventStream, rpcclient *BTCRPCClient) {
	fmt.Println("Syncing event streams. This may take a while...")
	for _, es := range ess {
		es.Sync(n, rpcclient)
		db.SaveEventStream(es)
	}
	fmt.Println("\nEvent streams synced.")
}

func handle_event(db *LocalDB, ev nostr.Event, rpcclient *BTCRPCClient) {
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
	es.Append(ev, rpcclient)
	db.SaveEventStream(es)

	printEvent(ev, &es.Name, true)
}
