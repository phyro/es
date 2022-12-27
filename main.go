package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/docopt/docopt-go"
	"github.com/nbd-wtf/go-nostr/nip19"
)

const USAGE = `es

Usage:
  es world
  es create <name> <key>
  es create <name> [--gen]
  es remove <name>
  es switch <name>
  es ll [-a]
  es append <content>
  es follow <name> <pubkey>
  es unfollow <name>
  es sync <name>
  es sync
  es log [--name=<name>]
  es show <id> [--verbose]
  es ots upgrade <name>
  es ots verify <name>
  es relay
  es relay add <url>
  es relay remove <url>
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
	nostr := Nostr{Relays: db.config.Relays}

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
		world(&db, nostr, all_es, verbose)

	// Event stream auth
	case opts["create"].(bool):
		// TODO: make this read from stdin and encrypt private key in jsons
		name := opts["<name>"].(string)
		priv_key, _ := opts.String("<key>")
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
		ev := findEvent(&db, nostr, id)
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
		ev, err := publishEvent(nostr, es_active.PrivKey, content, es_active.GetHead())
		if err != nil {
			log.Panic(err.Error())
		}
		es_active.Append(*ev)
		db.SaveEventStream(es_active)
	case opts["follow"].(bool):
		pubkey := nip19.TranslatePublicKey(opts["<pubkey>"].(string))
		name := opts["<name>"].(string)
		db.FollowEventStream(nostr, pubkey, name)
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
		es.Sync(nostr)
		db.SaveEventStream(es)

	// OpenTimestamps
	case opts["ots"].(bool):
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
		switch {
		case opts["upgrade"].(bool):
			es.OTSUpgrade()
			db.SaveEventStream(es)
		case opts["verify"].(bool):
			es.OTSVerify()
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
