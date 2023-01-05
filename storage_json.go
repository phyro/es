package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/nbd-wtf/go-nostr"
)

const STREAM_BASE_DIR = "~/.config/nostr/streams"
const STATE_FILE = "state.json"

// Very simple json storage
type LocalDB struct {
	state State
}

// Handles which event stream is active
type State struct {
	Active string `json:"active"`
}

func (s *State) Load() {
	base_dir_exp, _ := homedir.Expand(STREAM_BASE_DIR)
	os.Mkdir(base_dir_exp, 0700)
	path := filepath.Join(base_dir_exp, STATE_FILE)
	_, err := os.Open(path)
	if err != nil {
		// File doesn't exist, create it
		s = &State{Active: ""}
		s.Save()
		_, _ = os.Open(path)
	}
	f, _ := os.Open(path)
	err = json.NewDecoder(f).Decode(s)
	if err != nil {
		log.Fatal("can't parse state file " + path + ": " + err.Error())
	}
}

func (s *State) Save() {
	base_dir_exp, _ := homedir.Expand(STREAM_BASE_DIR)
	path := filepath.Join(base_dir_exp, STATE_FILE)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal("can't open state file " + path + ": " + err.Error())
		return
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(*s)
}

func (s *State) GetActive() string {
	return s.Active
}

func (s *State) SetActive(active string) {
	s.Active = active
}

/// StreamStore interface implementation

// Create a new event stream (or use an existing one)
func (db *LocalDB) CreateEventStream(name string, priv_key string, generate bool) {
	if priv_key == "" && !generate {
		log.Panic("You need to provide a private key or generate one when creating an account.")
	}
	if priv_key != "" && generate {
		log.Panic("You can't provide both a private key and generate one.")
	}
	key := ""
	seed := ""
	if priv_key != "" {
		key = priv_key
	}
	if generate {
		seed, key, _ = keyGen()
	}
	es := &EventStream{
		Name:    name,
		PrivKey: key,
		PubKey:  getPubKey(key),
		Log:     []nostr.Event{},
	}
	es.Print(false)
	fmt.Printf("\nSeed: %s \nPrivate key: %s", seed, key)

	err := db.SaveEventStream(es)
	if err != nil {
		log.Panic(err.Error())
	}
}

func (db *LocalDB) RemoveEventStream(name string) {
	pubkey, err := db.GetPubForName(name)
	if err != nil {
		log.Panic(err.Error())
	}
	es_active, _ := db.GetActiveStream()
	es, _ := db.GetEventStream(pubkey)
	is_active := es_active.PubKey == es.PubKey
	// If deleted user was active, set nobody to active
	if is_active {
		db.state.SetActive("")
		db.state.Save()
	}
	// Delete stream file
	path := pathForPubKey("stream", pubkey)
	e := os.Remove(path)
	if e != nil {
		log.Fatal(e)
	}
}

func (db *LocalDB) SetActiveEventStream(name string) error {
	pubkey, err := db.GetPubForName(name)
	if err != nil {
		return err
	}
	db.state.SetActive(pubkey)
	db.state.Save()

	return nil
}

// Get the active account
func (db *LocalDB) GetActiveStream() (*EventStream, error) {
	pubkey := db.state.GetActive()
	if pubkey == "" {
		return nil, errors.New("no active stream set")
	}
	return db.GetEventStream(pubkey)
}

// Get a specific event stream stored locally
func (db *LocalDB) GetEventStream(pubkey string) (*EventStream, error) {
	var es EventStream
	path := pathForPubKey("stream", pubkey)
	f, _ := os.Open(path)
	err := json.NewDecoder(f).Decode(&es)
	if err != nil {
		return nil, err
	}
	f.Close()

	return &es, nil
}

// Get all event streams stored locally
func (db *LocalDB) GetAllEventStreams() ([]*EventStream, error) {
	base_path, _ := homedir.Expand(STREAM_BASE_DIR)
	var result []*EventStream
	filepath.Walk(base_path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Fatalf(err.Error())
		}
		splt := strings.Split(info.Name(), ".")
		ext := splt[len(splt)-1]
		if info.Name() == "streams" || info.Name() == STATE_FILE || ext == "ots" {
			return nil
		}

		es, err := db.GetEventStream(strings.Split(info.Name(), ".")[0])
		if err != nil {
			log.Fatalf(err.Error())
		}
		result = append(result, es)
		return nil
	})

	return result, nil
}

func (db *LocalDB) SaveEventStream(es *EventStream) error {
	path := pathForPubKey("stream", es.PubKey)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal("can't open stream file " + path + ": " + err.Error())
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(es)

	return nil
}

// Follow a stream of a pubkey - we start at the genesis event (NULL)
func (db *LocalDB) FollowEventStream(n *Nostr, ots Timestamper, pubkey string, name string) error {
	if pubkey == "" {
		return errors.New("follow pubkey is empty")
	}
	if name == "" {
		return errors.New("name can't be empty")
	}

	es := &EventStream{
		Name:    name,
		PrivKey: "", // we don't own the stream, merely follow it
		PubKey:  pubkey,
		Log:     []nostr.Event{},
	}
	err := db.SaveEventStream(es)
	if err != nil {
		log.Panic(err.Error())
	}
	fmt.Printf("Followed %s.\n", pubkey)

	// Sync the event stream
	err = es.Sync(n, ots)
	if err != nil {
		return err
	}
	es.Print(false)
	db.SaveEventStream(es)

	return nil
}

// Unfollow a stream with a given name - equivalent to remove stream
func (db *LocalDB) UnfollowEventStream(name string) {
	db.RemoveEventStream(name)
}

func (db *LocalDB) ListEventStreams(include_followed bool) error {
	active_es, err := db.GetActiveStream()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	owned, followed := db.GetOwnedFollowedESS()
	for _, es := range owned {
		if es.PubKey == active_es.PubKey {
			fmt.Printf("* ")
		}
		es.Print(false)
	}
	if include_followed {
		fmt.Printf("\n------------------------------------\n")
		fmt.Printf("Following:")
		fmt.Printf("\n------------------------------------\n")
		for _, es := range followed {
			es.Print(false)
		}
	}

	return nil
}

// Returns two lists: owned and followed events streams
func (db *LocalDB) GetOwnedFollowedESS() ([]*EventStream, []*EventStream) {
	ess, err := db.GetAllEventStreams()
	if err != nil {
		log.Panic(err.Error())
	}
	owned := make([]*EventStream, 0)
	for _, es := range ess {
		if es.PrivKey != "" {
			owned = append(owned, es)
		}
	}

	// We follow every stream in "GetAllEventStreams" hence why ess is the result
	return owned, ess
}

// Returns a public key associated with the given name
func (db *LocalDB) GetPubForName(name string) (string, error) {
	found := false
	rv := ""
	ess, err := db.GetAllEventStreams()
	if err != nil {
		return "", err
	}
	for _, es := range ess {
		if es.Name == name {
			if found {
				return "", fmt.Errorf("name conflict for name: %s\npub1: %s\npub2: %s", name, rv, es.PubKey)
			}
			rv = es.PubKey
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("could not find stream with name: %s", name)
	}
	return rv, nil
}

/// Utils

// Given a context (i.e. "account", "stream") returns path to file
func pathForPubKey(ctx string, pubkey string) string {
	base_path, _ := homedir.Expand(STREAM_BASE_DIR)
	path := filepath.Join(base_path, pubkey) + "." + ctx + ".json"
	return path
}
