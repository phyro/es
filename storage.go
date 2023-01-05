package main

import (
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/phyro/go-opentimestamps/opentimestamps"
)

type StreamService struct {
	store  StreamStore
	config *Config
	ots    *OTSService
}

func (s *StreamService) Load() {
	cfg := Config{}
	cfg.Load()
	store := LocalDB{}
	store.state.Load()

	s.store = &store
	s.config = &cfg
	s.ots = &OTSService{rpcclient: cfg.GetBitcoinRPC()}
}

// A StreamStore provides an interface for managing EventStreams including timestamping
type StreamStore interface {
	StreamStoreReader
	StreamStoreWriter
}

// Handling all read operations on stream store
type StreamStoreReader interface {
	GetActiveStream() (*EventStream, error)
	GetEventStream(string) (*EventStream, error)
	GetAllEventStreams() ([]*EventStream, error)
	// Misc
	ListEventStreams(bool) error
	GetPubForName(string) (string, error)
}

// Handling all write operations on stream store
type StreamStoreWriter interface {
	CreateEventStream(string, string, bool)
	SaveEventStream(*EventStream) error
	RemoveEventStream(string)
	SetActiveEventStream(string) error
	FollowEventStream(*Nostr, Timestamper, string, string) error
	UnfollowEventStream(string)
}

// An EventStreamer provides an interface for managing event streams
type EventStreamer interface {
	// Core event stream behaviour
	Create(string, Timestamper) (*nostr.Event, error)
	Append(nostr.Event, Timestamper) error
	Sync(*Nostr, Timestamper) error
	Size() int
	GetHead() string

	// Relay management
	AddRelay(string)
	RemoveRelay(string) error
	ListRelays() []string

	Print(bool)
	// TODO: We can make a correct by construction design by appending only
	// valid events that follow the rules. What happens if the calendar doesn't
	// attest to our event though? We may need a "Verify" on EventStreamer
}

// Timestamper handles the timestamping of events
type Timestamper interface {
	// Should return a b64 string
	Stamp(*nostr.Event) (string, error)
	IsUpgraded(*nostr.Event) bool
	Upgrade(*nostr.Event) (*opentimestamps.Timestamp, error)
	Verify(*nostr.Event) (bool, *time.Time, error)
	HasRPCConfigured() bool
}

type Relayer interface {
	AddRelay(string)
	RemoveRelay(string)
	ListRelays() []string
}

type BitcoinRPCManager interface {
	ConfigureBitcoinRPC(string, string, string) error
	UnsetBitcoinRPC()
	GetBitcoinRPC() *BTCRPCClient
}
