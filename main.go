package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/docopt/docopt-go"
)

const USAGE = `es

Usage:
  es world
  es create <name> <privkey>
  es create <name> [--gen]
  es remove <name>
  es switch <name>
  es ll [-a]
  es append <content>
  es follow <name> <pubkey>
  es unfollow <name>
  es sync <name>
  es sync
  es push <name> <url>
  es push
  es log [--name=<name>]
  es show <id> [--verbose]
  es ots upgrade <name>
  es ots verify <name>
  es ots rpc <url> <user> <password>
  es ots norpc
  es relay
  es relay add <url>
  es relay remove <url>

All pubkeys passed should *NOT* be bech32 encoded.
`

// Fails if we have no active event stream (required for appending etc.)
func require_active(store StreamStore) {
	_, err := store.GetActiveStream()
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

func require_relays(es *EventStream) {
	if len(es.ListRelays()) == 0 {
		log.Println("this event stream has no relays set")
		os.Exit(1)
	}
}

func main() {
	flag.Parse()
	log.SetPrefix("<> ")

	srv := &StreamService{}
	srv.Load()
	// Add a single default relay so the pool isn't empty
	n := NewNostr([]string{"wss://nostr-relay.digitalmob.ro"})

	// Parse args
	opts, err := docopt.ParseArgs(USAGE, flag.Args(), "")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// Event stream auth commands - don't require an active event stream set
	switch {
	case opts["create"].(bool):
		// TODO: make this read from stdin and encrypt private key in jsons
		name := opts["<name>"].(string)
		priv_key, _ := opts.String("<privkey>")
		generate, _ := opts.Bool("--gen")
		srv.store.CreateEventStream(name, priv_key, generate)
	// We have to check that the "remove" option is not called with "es relay remove"
	case opts["remove"].(bool) && !opts["relay"].(bool):
		name := opts["<name>"].(string)
		srv.store.RemoveEventStream(name)
		fmt.Printf("Removed %s stream.", name)
	case opts["switch"].(bool):
		name := opts["<name>"].(string)
		srv.store.SetActiveEventStream(name)
	case opts["ll"].(bool):
		require_active(srv.store)
		all, _ := opts.Bool("-a")
		srv.store.ListEventStreams(all)
	}

	es_active, err := srv.store.GetActiveStream()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	n.AddRelays(es_active.Relays)

	switch {
	// View the event stream world
	case opts["world"].(bool):
		verbose, _ := opts.Bool("--verbose")
		all_es, err := srv.store.GetAllEventStreams()
		if err != nil {
			log.Panic(err.Error())
		}
		world(srv, n, all_es, verbose)
	// View
	case opts["log"].(bool):
		require_active(srv.store)
		pubkey := es_active.PubKey
		if val, _ := opts["--name"]; val != nil {
			pubkey, _ = srv.store.GetPubForName(val.(string))
		}
		es, _ := srv.store.GetEventStream(pubkey)
		es.Print(true)
	case opts["show"].(bool):
		require_active(srv.store)
		require_relays(es_active)
		verbose, _ := opts.Bool("--verbose")
		id := opts["<id>"].(string)
		if id == "" {
			log.Println("provided event ID was empty")
			return
		}
		ev, err := findEvent(n, id)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		// Check if we have a name for the event stream owner
		var name *string
		es, err := srv.store.GetEventStream(ev.PubKey)
		if err == nil {
			name = &es.Name
		}
		printEvent(*ev, name, verbose)

	// Core
	case opts["append"].(bool):
		require_active(srv.store)
		require_relays(es_active)
		content := opts["<content>"].(string)
		ev, err := es_active.Create(content, srv.ots)
		if err != nil {
			log.Panic(err.Error())
		}
		err = n.BroadcastEvent(es_active.Relays, *ev)
		if err != nil {
			log.Println(err.Error())
			// Even if we failed to broadcast, we still save the event
		}
		srv.store.SaveEventStream(es_active)
		fmt.Println("Added event:", ev.ID)
	case opts["follow"].(bool):
		require_active(srv.store)
		pubkey := opts["<pubkey>"].(string)
		name := opts["<name>"].(string)
		err := srv.store.FollowEventStream(n, srv.ots, pubkey, name)
		if err != nil {
			log.Panic(err.Error())
		} else {
			fmt.Println("ok")
		}
	case opts["unfollow"].(bool):
		require_active(srv.store)
		name := opts["<name>"].(string)
		srv.store.UnfollowEventStream(name)
		fmt.Printf("Removed %s stream.", name)
	case opts["sync"].(bool):
		require_relays(es_active)
		require_active(srv.store)
		es, _ := srv.store.GetActiveStream()
		if val, _ := opts["<name>"]; val != nil {
			pubkey, _ := srv.store.GetPubForName(val.(string))
			es, _ = srv.store.GetEventStream(pubkey)
		}
		err := es.Sync(n, srv.ots)
		// We save first as we might have added a few new valid events before error
		srv.store.SaveEventStream(es)
		if err != nil {
			log.Panic(err.Error())
		}

	case opts["push"].(bool):
		require_active(srv.store)
		name := opts["<name>"].(string)
		relayUrl := opts["<url>"].(string)
		pubkey, _ := srv.store.GetPubForName(name)
		es, err := srv.store.GetEventStream(pubkey)
		if err != nil {
			log.Panic(err.Error())
		}
		fmt.Printf("Pushing stream labeled as %s to relay %s\n", name, relayUrl)
		// Create a new nostr relay pool with just this relay
		nPush := NewNostr([]string{relayUrl})
		err = es.Mirror(nPush, relayUrl)
		if err != nil {
			log.Println(err.Error())
			return
		}
		// We save because we added the relay to the list of relays of this stream
		srv.store.SaveEventStream(es)
		fmt.Println("Stream succesfully pushed.")

	// OpenTimestamps
	case opts["ots"].(bool):
		require_active(srv.store)
		switch {
		case opts["upgrade"].(bool):
			name := opts["<name>"].(string)
			pubkey, err := srv.store.GetPubForName(name)
			if err != nil {
				log.Println(err.Error())
				return
			}
			es, err := srv.store.GetEventStream(pubkey)
			if err != nil {
				log.Println(err.Error())
				return
			}
			err = es.OTSUpgrade()
			if err != nil {
				log.Println(err.Error())
			}
			srv.store.SaveEventStream(es)
		case opts["verify"].(bool):
			name := opts["<name>"].(string)
			pubkey, err := srv.store.GetPubForName(name)
			if err != nil {
				log.Println(err.Error())
				return
			}
			es, err := srv.store.GetEventStream(pubkey)
			if err != nil {
				log.Println(err.Error())
				return
			}
			es.OTSVerify(srv.ots)
		case opts["rpc"].(bool):
			host := opts["<url>"].(string)
			user := opts["<user>"].(string)
			password := opts["<password>"].(string)
			err := srv.config.ConfigureBitcoinRPC(host, user, password)
			if err != nil {
				log.Printf("\nCould not connect, keeping old rpc settings. Error: %s", err.Error())
				return
			}
			fmt.Println("Successfully configured Bitcoin RPC.")
		case opts["norpc"].(bool):
			srv.config.UnsetBitcoinRPC()
		}

	// Relay
	case opts["relay"].(bool):
		require_active(srv.store)
		switch {
		case opts["add"].(bool):
			url := opts["<url>"].(string)
			err := es_active.AddRelay(url)
			if err != nil {
				log.Println(err.Error())
				return
			}
			srv.store.SaveEventStream(es_active)
			fmt.Println("Relay added")
		case opts["remove"].(bool):
			url := opts["<url>"].(string)
			err := es_active.RemoveRelay(url)
			if err != nil {
				log.Println(err.Error())
				return
			}
			srv.store.SaveEventStream(es_active)
			fmt.Println("Relay removed")
		default:
			for _, relay_url := range es_active.ListRelays() {
				fmt.Println("Url:", relay_url)
			}
		}
	}
}
