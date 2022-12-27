package main

import (
	b64 "encoding/base64"
	"fmt"
	"log"

	"github.com/nbd-wtf/go-nostr"
)

const GENESIS = "NULL"

type EventStream struct {
	Name    string        `json:"name"`
	PrivKey string        `json:"privkey"`
	PubKey  string        `json:"pubkey"`
	Log     []nostr.Event `json:"log"`
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
	// Verify "prev" of the new event matches the last event id
	last_event_id := es.Log[len(es.Log)-1].ID
	prev := get_prev(ev)
	if prev != last_event_id {
		log.Printf("Reference to previous event mismatch. Last event id: %s, prev: %s", last_event_id, prev)
	}

	// Stamp with ots
	ots_content := stamp(&ev)
	ots_b64 := b64.StdEncoding.EncodeToString([]byte(ots_content))
	ev.SetExtra("ots", ots_b64)

	// Aal izz well, append event to the stream
	es.Log = append(es.Log, ev)
}

// Sync a stream - find the latest HEAD and query for the next event of the stream and repeat
func (es *EventStream) Sync(n Nostr) {
	fmt.Printf("Syncing %s ... ", es.Name)
	prev := es.GetHead()

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

func (es *EventStream) OTSUpgrade() {
	for _, ev := range es.Log {
		if !is_ots_upgraded(&ev) {
			fmt.Printf("\nUpgrading OTS for event id: %s", ev.ID)
			upgraded_ots, err := ots_upgrade(&ev)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			upgraded_ots_b64 := b64.StdEncoding.EncodeToString([]byte(upgraded_ots))
			ev.SetExtra("ots", upgraded_ots_b64)
		}
	}
}

func (es *EventStream) OTSVerify() {
	// // First try to upgrade all OTS
	// es.OTSUpgrade()

	// for _, ev := range es.Log {
	// 	fmt.Printf("\nVerifying OTS for event id: %s ;", ev.ID)
	// 	if is_ots_upgraded(&ev) {
	// 		ok, err := ots_verify(&ev)
	// 		if err != nil {
	// 			fmt.Printf("FAIL (error): %s", err.Error())
	// 		}
	// 		if ok {
	// 			fmt.Printf("SUCCESS")
	// 		} else {
	// 			fmt.Printf("FAIL (verification failed)")
	// 		}
	// 	} else {
	// 		fmt.Printf("FAIL (not upgraded)")
	// 	}
	// }

	// TMP
	for _, ev := range es.Log {
		ok, err := ots_verify_direct(&ev)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			if ok {
				fmt.Println("successfully verified")
			} else {
				fmt.Println("failed verification")
			}
		}
	}
}
