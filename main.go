package main

import (
	"flag"
	"fmt"
	"log"

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
func require_active(db *LocalDB) {
	if db.config.Active == "" {
		log.Panic("Please set an active stream with `es switch <name>`.")
	}
}

func main() {
	flag.Parse()
	log.SetPrefix("<> ")

	db := LocalDB{}
	db.Init()
	n := NewNostr(db.config.Relays)

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
		all_es, err := db.GetAllEventStreams()
		if err != nil {
			log.Panic(err.Error())
		}
		world(&db, n, all_es, verbose, db.config.BTCRPC)

	// Event stream auth
	case opts["create"].(bool):
		// TODO: make this read from stdin and encrypt private key in jsons
		name := opts["<name>"].(string)
		priv_key, _ := opts.String("<privkey>")
		generate, _ := opts.Bool("--gen")
		db.CreateEventStream(name, priv_key, generate)
	case opts["remove"].(bool):
		name := opts["<name>"].(string)
		db.RemoveEventStream(name)
		fmt.Printf("Removed %s stream.", name)
	case opts["switch"].(bool):
		name := opts["<name>"].(string)
		db.SetActiveEventStream(name)
	case opts["ll"].(bool):
		require_active(&db)
		all, _ := opts.Bool("-a")
		db.ListEventStreams(all)

	// View
	case opts["log"].(bool):
		require_active(&db)
		es_active, _ := db.GetActiveStream()
		pubkey := es_active.PubKey
		if val, _ := opts["--name"]; val != nil {
			pubkey, _ = db.GetPubForName(val.(string))
		}
		es, _ := db.GetEventStream(pubkey)
		es.Print(true)
	case opts["show"].(bool):
		verbose, _ := opts.Bool("--verbose")
		id := opts["<id>"].(string)
		if id == "" {
			log.Println("provided event ID was empty")
			return
		}
		ev := findEvent(&db, n, id)
		// Check if we have a name for the event stream owner
		var name *string
		es, err := db.GetEventStream(ev.PubKey)
		if err == nil {
			name = &es.Name
		}
		printEvent(*ev, name, verbose)

	// Core
	case opts["append"].(bool):
		require_active(&db)
		es_active, _ := db.GetActiveStream()
		content := opts["<content>"].(string)
		ev, err := es_active.Create(content, db.config.BTCRPC)
		if err != nil {
			log.Panic(err.Error())
		}
		err = publishEvent(n, ev)
		if err != nil {
			log.Panic(err.Error())
		}
		db.SaveEventStream(es_active)
	case opts["follow"].(bool):
		pubkey := opts["<pubkey>"].(string)
		name := opts["<name>"].(string)
		err := db.FollowEventStream(n, pubkey, name, db.config.BTCRPC)
		if err != nil {
			log.Panic(err.Error())
		} else {
			fmt.Println("ok")
		}
	case opts["unfollow"].(bool):
		name := opts["<name>"].(string)
		db.UnfollowEventStream(name)
		fmt.Printf("Removed %s stream.", name)
	case opts["sync"].(bool):
		require_active(&db)
		es, _ := db.GetActiveStream()
		if val, _ := opts["<name>"]; val != nil {
			pubkey, _ := db.GetPubForName(val.(string))
			es, _ = db.GetEventStream(pubkey)
		}
		err := es.Sync(n, db.config.BTCRPC)
		// We save first as we might have added a few new valid events before error
		db.SaveEventStream(es)
		if err != nil {
			log.Panic(err.Error())
		}

	case opts["push"].(bool):
		name := opts["<name>"].(string)
		pubkey, _ := db.GetPubForName(name)
		es, err := db.GetEventStream(pubkey)
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
			pubkey, err := db.GetPubForName(name)
			if err != nil {
				log.Println(err.Error())
				return
			}
			es, err := db.GetEventStream(pubkey)
			if err != nil {
				log.Println(err.Error())
				return
			}
			err = es.OTSUpgrade()
			if err != nil {
				log.Println(err.Error())
			}
			db.SaveEventStream(es)
		case opts["verify"].(bool):
			name := opts["<name>"].(string)
			pubkey, err := db.GetPubForName(name)
			if err != nil {
				log.Println(err.Error())
				return
			}
			es, err := db.GetEventStream(pubkey)
			if err != nil {
				log.Println(err.Error())
				return
			}
			es.OTSVerify(db.config.BTCRPC)
		case opts["rpc"].(bool):
			host := opts["<url>"].(string)
			user := opts["<user>"].(string)
			password := opts["<password>"].(string)
			err := db.ConfigureBitcoinRPC(host, user, password)
			if err != nil {
				log.Printf("\nCould not connect, keeping old rpc settings. Error: %s", err.Error())
				return
			}
			db.SaveConfig()
			// TODO: add a ping to test
			fmt.Println("Successfully configured Bitcoin RPC.")
		case opts["norpc"].(bool):
			db.UnsetBitcoinRPC()
			db.SaveConfig()
		}

	// Relay
	case opts["relay"].(bool):
		switch {
		case opts["add"].(bool):
			url := opts["<url>"].(string)
			db.AddRelay(url)
		case opts["remove"].(bool):
			url := opts["<url>"].(string)
			db.RemoveRelay(url)
		default:
			db.ListRelays()
		}
	}
}
