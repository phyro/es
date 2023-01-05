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
  es push <name>
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

func main() {
	flag.Parse()
	log.SetPrefix("<> ")

	srv := &StreamService{}
	srv.Load()
	require_active(srv.store)
	es_active, err := srv.store.GetActiveStream()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	n, err := NewNostr(es_active.Relays)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// Parse args
	opts, err := docopt.ParseArgs(USAGE, flag.Args(), "")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	switch {
	// View the event stream world
	case opts["world"].(bool):
		verbose, _ := opts.Bool("--verbose")
		all_es, err := srv.store.GetAllEventStreams()
		if err != nil {
			log.Panic(err.Error())
		}
		world(srv, n, all_es, verbose)

	// Event stream auth
	case opts["create"].(bool):
		// TODO: make this read from stdin and encrypt private key in jsons
		name := opts["<name>"].(string)
		priv_key, _ := opts.String("<privkey>")
		generate, _ := opts.Bool("--gen")
		srv.store.CreateEventStream(name, priv_key, generate)
	case opts["remove"].(bool):
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

	// View
	case opts["log"].(bool):
		pubkey := es_active.PubKey
		if val, _ := opts["--name"]; val != nil {
			pubkey, _ = srv.store.GetPubForName(val.(string))
		}
		es, _ := srv.store.GetEventStream(pubkey)
		es.Print(true)
	case opts["show"].(bool):
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
		content := opts["<content>"].(string)
		ev, err := es_active.Create(content, srv.ots)
		if err != nil {
			log.Panic(err.Error())
		}
		err = publishEvent(n, ev)
		if err != nil {
			log.Panic(err.Error())
		}
		srv.store.SaveEventStream(es_active)
		fmt.Println("Added event:", ev.ID)
	case opts["follow"].(bool):
		pubkey := opts["<pubkey>"].(string)
		name := opts["<name>"].(string)
		err := srv.store.FollowEventStream(n, srv.ots, pubkey, name)
		if err != nil {
			log.Panic(err.Error())
		} else {
			fmt.Println("ok")
		}
	case opts["unfollow"].(bool):
		name := opts["<name>"].(string)
		srv.store.UnfollowEventStream(name)
		fmt.Printf("Removed %s stream.", name)
	case opts["sync"].(bool):
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
		name := opts["<name>"].(string)
		pubkey, _ := srv.store.GetPubForName(name)
		es, err := srv.store.GetEventStream(pubkey)
		if err != nil {
			log.Panic(err.Error())
		}
		fmt.Printf("Pushing stream labeled as %s\n", name)
		err = publishStream(n, es)
		if err != nil {
			log.Panic(err.Error())
		}
		fmt.Println("Stream succesfully pushed.")

	// OpenTimestamps
	case opts["ots"].(bool):
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
