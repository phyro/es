package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type Nostr struct {
	// Holds a pool of relays based on their url
	Pool map[string]*nostr.Relay
}

func NewNostr(relays []string) *Nostr {
	n := &Nostr{Pool: map[string]*nostr.Relay{}}
	for _, relayUrl := range relays {
		n.AddRelay(relayUrl)
	}
	return n
}

func (n *Nostr) AddRelay(url string) error {
	if _, ok := n.Pool[url]; !ok {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		r, err := nostr.RelayConnect(ctx, url)
		if err != nil {
			return err
		}
		n.Pool[url] = r
	}

	return nil
}

func (n *Nostr) AddRelays(rs []string) error {
	for _, relayUrl := range rs {
		err := n.AddRelay(relayUrl)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *Nostr) SingleQuery(relayUrl string, filter nostr.Filter) ([]nostr.Event, error) {
	r, ok := n.Pool[relayUrl]
	if !ok {
		return nil, fmt.Errorf("relay url %s not in the pool", relayUrl)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.QuerySync(ctx, filter), nil
}

// Query the whole pool for a specific filter - useful for finding events we don't know where to find
func (n *Nostr) SingleQueryPool(filter nostr.Filter) ([]nostr.Event, error) {
	if len(n.Pool) == 0 {
		return []nostr.Event{}, errors.New("relay pool is empty")
	}
	evsAggregator := make(chan []nostr.Event)
	var wg sync.WaitGroup

	// TODO: Figure out what to do if the pool has too many relays
	for relayUrl, _ := range n.Pool {
		wg.Add(1)
		go func() {
			defer wg.Done()
			evs, err := n.SingleQuery(relayUrl, filter)
			if err == nil && len(evs) > 0 {
				evsAggregator <- evs
			}
		}()
	}
	wg.Wait()

	// We probably got a lot of the same events from different relays. Make a unique list
	seen := map[string]bool{}
	result := []nostr.Event{}
	for evs := range evsAggregator {
		for _, ev := range evs {
			if _, ok := seen[ev.ID]; !ok {
				result = append(result, ev)
				seen[ev.ID] = true
			}
		}
	}

	return result, nil
}

func (n *Nostr) Listen(wg *sync.WaitGroup, ctx context.Context, evt_chan chan nostr.Event, filter nostr.Filter) {
	// TODO: You don't need to listen for all keys on all relays
	// TODO: Make events unique. We don't want to send the same event twice
	for _, r := range n.Pool {
		sub := r.Subscribe(ctx, nostr.Filters{filter})
		// Initiate subscriptions in go threads and delegate results to event channel
		fmt.Printf("\nStarted sub on relay: %s", r.URL)
		wg.Add(1)
		go func(relay_url string) {
			defer wg.Done()
			for {
				select {
				case ev := <-sub.Events:
					evt_chan <- ev
				case <-ctx.Done():
					sub.Unsub()
					fmt.Printf("\nClosing sub for relay: %s", relay_url)
					return
				}
			}
		}(r.URL)
	}
	fmt.Println()
}

// Broadcasts event to all the relays specified
func (n *Nostr) BroadcastEvent(relayUrls []string, ev nostr.Event) error {
	gotErr := false
	for _, relayUrl := range relayUrls {
		status, err := n.SendEvent(relayUrl, ev)
		if err != nil {
			log.Printf("Error: event: %s to relay %s. Error: %w", ev.ID, relayUrl, err)
			gotErr = true
		}
		if status != 1 {
			log.Printf("Error publishing event: %s to relay %s. Status: %s", ev.ID, relayUrl, status)
			gotErr = true
		}
	}
	if gotErr {
		return errors.New("please check the errors")
	}

	return nil
}

func (n *Nostr) SendEvent(relayUrl string, ev nostr.Event) (nostr.Status, error) {
	r, ok := n.Pool[relayUrl]
	if !ok {
		return nostr.PublishStatusFailed, fmt.Errorf("relay url %s not in the pool", relayUrl)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.Publish(ctx, ev), nil
}
