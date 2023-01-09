package main

import (
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/phyro/go-opentimestamps/opentimestamps"
)

// StreamStore provides an interface for storage and retrieval of EventStreams
type StreamStore interface {
	StreamStoreReader
	StreamStoreWriter
}

type StreamStoreReader interface {
	GetActiveStream() (*EventStream, error)
	GetEventStream(string) (*EventStream, error)
	GetAllEventStreams() ([]*EventStream, error)
	// Misc
	ListEventStreams(bool) error
	GetPubForName(string) (string, error)
}

type StreamStoreWriter interface {
	CreateEventStream(string, string, bool)
	SaveEventStream(*EventStream) error
	RemoveEventStream(string)
	SetActiveEventStream(string) error
	FollowEventStream(*Nostr, Timestamper, string, string) error
	UnfollowEventStream(string)
}

// EventStreamer provides an interface for managing event streams
type EventStreamer interface {
	EventStreamReader
	EventStreamWriter
	Print(bool)
	// TODO: We can make a correct by construction design by appending only
	// valid events that follow the rules. What happens if the calendar doesn't
	// attest to our event though? We may need a "Verify" on EventStreamer
}

type EventStreamReader interface {
	Size() int
	GetHead() string
	ListRelays() []string
	HasRelays() bool
}

type EventStreamWriter interface {
	Create(string, Timestamper) (*nostr.Event, error)
	Append(nostr.Event, Timestamper) error
	Sync(*Nostr, Timestamper) error
	Mirror(*Nostr, string) error
	AddRelay(string) error
	RemoveRelay(string) error
}

// Timestamper provides an interface for timestamping nostr events
type Timestamper interface {
	Stamp(*nostr.Event) (string, error)
	IsUpgraded(*nostr.Event) bool
	Upgrade(*nostr.Event) (*opentimestamps.Timestamp, error)
	Verify(*nostr.Event) (bool, *time.Time, error)
	HasRPCConfigured() bool
}

type BitcoinRPCManager interface {
	ConfigureBitcoinRPC(string, string, string) error
	UnsetBitcoinRPC()
	GetBitcoinRPC() *BTCRPCClient
}
