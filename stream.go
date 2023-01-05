package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

const GENESIS = "NULL"

type BTCRPCClient struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type EventStream struct {
	Name    string        `json:"name"`
	PrivKey string        `json:"privkey"`
	PubKey  string        `json:"pubkey"`
	Relays  []string      `json:"relays"`
	Log     []nostr.Event `json:"log"`
}

func (es *EventStream) Create(content string, ots Timestamper) (*nostr.Event, error) {
	if es.PrivKey == "" {
		return nil, fmt.Errorf("can't create an event. No private key for this stream is set")
	}
	prev := es.GetHead()
	tags := nostr.Tags{nostr.Tag{"prev", prev}}

	event := &nostr.Event{
		CreatedAt: time.Now(),
		Kind:      nostr.KindTextNote,
		Tags:      tags,
		Content:   content,
		PubKey:    es.PubKey,
	}

	// Sign the event
	err := event.Sign(es.PrivKey)
	if err != nil {
		return nil, fmt.Errorf("error signing event: %w", err)
	}

	// Stamp with ots
	ots_b64, err := ots.Stamp(event)
	if err != nil {
		return nil, fmt.Errorf("Event stamping error: %v", err)
	}
	event.SetExtra("ots", ots_b64)

	// We append the event as soon as it is created. This verifies all the event stream properties are present
	err = es.Append(*event, ots)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func (es *EventStream) Append(ev nostr.Event, ots Timestamper) error {
	// Check pubkey and verify signature
	if ev.PubKey != es.PubKey {
		return fmt.Errorf("can't append event from pubkey %s to stream with pubkey %s", ev.PubKey, es.PubKey)
	}
	ok, err := ev.CheckSignature()
	if err != nil {
		log.Printf("invalid signature for event: %s", ev.ID)
		return err
	}
	if !ok {
		// According to code, if the signature is not valid 'ok' will be false
		return fmt.Errorf("signature verification failed for event: %s", ev.ID)
	}
	if es.Size() > 0 {
		// Verify "prev" of the new event matches the last event id
		last_event_id := es.Log[len(es.Log)-1].ID
		prev := get_prev(ev)
		if prev != last_event_id {
			return fmt.Errorf("reference to previous event mismatch. Last event id: %s, prev: %s", last_event_id, prev)
		}
	}

	// Verifying "ots" before appending gives us a guarantee that every stream will have attestations
	// Additonally, we check if the attestation is linear in case we get attested time.
	if ev.GetExtraString("ots") == "" {
		return fmt.Errorf("event is missing the \"ots\" field")
	}
	is_good, attested_time, err := ots.Verify(&ev)
	if !is_good {
		return err
	} else {
		if attested_time != nil {
			// Check that it builds on the previous event
			last_event := es.Log[len(es.Log)-1]
			// TODO: We should check attestation time, not event.created_at field
			if attested_time.Before(last_event.CreatedAt) {
				return fmt.Errorf("attested time is after the last event created at")
			}
		}
	}

	// Append event to the stream
	es.Log = append(es.Log, ev)

	return nil
}

// Sync a stream - find the latest HEAD and query for the next event of the stream and repeat
func (es *EventStream) Sync(n *Nostr, ots Timestamper) error {
	fmt.Printf("Syncing %s ... ", es.Name)
	prev := es.GetHead()
	num_new := 0
	// Start from the genesis event and iterate forward
	for {
		events, err := findNextEvents(n, es.PubKey, prev)
		if err != nil {
			fmt.Println(err.Error())
			break
		}
		if len(events) == 0 {
			break
		}
		num_new += len(events)
		for _, ev := range events {
			err = es.Append(*ev, ots)
			if err != nil {
				return err
			}
			prev = ev.ID
		}
	}
	fmt.Printf("Done\nNumber of new events: %d\nHEAD (%s) at: %s", num_new, es.Name, es.GetHead())

	return nil
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

func (es *EventStream) AddRelay(url string) error {
	for _, entry := range es.ListRelays() {
		if url == entry {
			return errors.New("relay was already added")
		}
	}
	es.Relays = append(es.Relays, url)

	return nil
}

func (es *EventStream) RemoveRelay(url string) error {
	result := []string{}
	found := false
	for _, relay_url := range es.Relays {
		if relay_url != url {
			result = append(result, relay_url)
		} else {
			found = true
		}
	}
	if !found {
		return errors.New("relay url was not on the list")
	} else {
		es.Relays = result
	}

	return nil
}

func (es *EventStream) ListRelays() []string {
	return es.Relays
}

func (es *EventStream) Print(show_chain bool) {
	fmt.Printf("%s (%s)\n", es.Name, es.PubKey)
	if !show_chain {
		return
	}
	indent := "\t\t\t"
	fmt.Printf("\nEvent stream:\n")
	fmt.Printf("----------------------------------------------------------\n")
	fmt.Printf("%s%s", indent, GENESIS)
	fmt.Printf("\n----------------------------------------------------------\n")
	if es.Size() == 0 {
		return
	}

	fmt.Printf("%s|", indent)
	fmt.Printf("\n%sv\n", indent)
	for idx, event := range es.Log {
		fmt.Printf("----------------------------------------------------------\n")
		printEvent(event, &es.Name, true)
		fmt.Printf("\n----------------------------------------------------------\n")
		if idx != es.Size()-1 {
			fmt.Printf("%s|", indent)
			fmt.Printf("\n%sv\n", indent)
		}
	}
}

func (es *EventStream) OTSUpgrade() error {
	return errors.New("not implemented")
}

// Verifies two things:
// 1. Every event must have an attestation
// 2. Events must have linear attested time
func (es *EventStream) OTSVerify(ots Timestamper) {
	last_attestation_time := time.Time{}
	last_attestation_event_id := "/"
	for _, ev := range es.Log {
		is_good, attested_time, err := ots.Verify(&ev)
		printOTSResult(&ev, is_good, attested_time, err)
		if attested_time != nil {
			if attested_time.Before(last_attestation_time) {
				fmt.Printf("\nError: Nonlinear attestations found.")
				fmt.Printf("\nLast event id: %s", last_attestation_event_id)
				fmt.Printf("Event id: %s", ev.ID)
			} else {
				last_attestation_time = *attested_time
				last_attestation_event_id = ev.ID
			}
		}
	}
	if !ots.HasRPCConfigured() {
		fmt.Println("\nNOTE: In case you don't trust blockchain.info, verify the merkle root hashes manually.")
	}
}

func printOTSResult(ev *nostr.Event, is_good bool, attested_time *time.Time, err error) {
	status := "FAIL"
	if is_good {
		status = "OK"
	}
	if err != nil {
		if err == ErrOTSPending {
			fmt.Printf("\nEvent id: %s: Status: %s (PENDING)", ev.ID, status)
		} else if err == ErrOTSWaitingConfirmations {
			fmt.Printf("\nEvent id: %s: Status: %s (WAITING 5 CONFIRMATIONS)", ev.ID, status)
		} else {
			fmt.Printf("\nEvent id: %s: Status: %s (UKNOWN). Error: %s", ev.ID, status, err.Error())
		}
	} else {
		if attested_time != nil {
			fmt.Printf("\nEvent id: %s: Status: %s (%s)", ev.ID, status, attested_time)
		} else {
			fmt.Printf("\nEvent id: %s: Status: %s", ev.ID, status)
		}
	}
}
