package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/nbd-wtf/go-nostr"
)

func world(srv *StreamService, n *Nostr, event_streams []*EventStream, verbose bool) {
	if len(event_streams) == 0 {
		log.Println("You need to be following at least one stream to run 'world'")
		return
	}

	// We can't sync or listen if we don't know where
	ess_filtered := []*EventStream{}
	for _, es := range event_streams {
		if es.HasRelays() {
			ess_filtered = append(ess_filtered, es)
		}
	}

	// Before listening, we have to sync all event streams to their HEAD
	sync_all(srv.store, n, srv.ots, ess_filtered)
	var keys []string
	for _, es := range ess_filtered {
		keys = append(keys, es.PubKey)
	}

	// Find events for the streams we follow
	cancel_chan := make(chan os.Signal, 1)
	signal.Notify(cancel_chan, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	evt_chan := make(chan nostr.Event)
	n.Listen(&wg, ctx, evt_chan, nostr.Filter{Authors: keys})
L:
	for {
		select {
		case ev := <-evt_chan:
			handle_event(srv.store, srv.ots, ev)
		case sig := <-cancel_chan:
			fmt.Println(sig)
			// Shutdown threads
			cancel()
			done <- true
			break L
		}
	}

	<-done
	wg.Wait()
	fmt.Println("\nBye world.")
}

func sync_all(store StreamStore, n *Nostr, ots Timestamper, ess []*EventStream) {
	fmt.Println("Syncing event streams. This may take a while...")
	for _, es := range ess {
		es.Sync(n, ots)
		store.SaveEventStream(es)
	}
	fmt.Println("\nEvent streams synced.")
}

func handle_event(store StreamStore, ots Timestamper, ev nostr.Event) {
	// Find the expected head of the event stream
	es, err := store.GetEventStream(ev.PubKey)
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
	es.Append(ev, ots)
	store.SaveEventStream(es)

	printEvent(ev, &es.Name, true)
}
