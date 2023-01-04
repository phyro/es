package main

type StreamService struct {
	store  StreamStore
	config *Config
}

func (s *StreamService) Load() {
	cfg := Config{}
	cfg.Load()
	store := LocalDB{}
	store.state.Load()

	s.store = &store
	s.config = &cfg
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
	FollowEventStream(*Nostr, string, string, *BTCRPCClient) error
	UnfollowEventStream(string)
}

type Relayer interface {
	AddRelay(string)
	RemoveRelay(string)
	ListRelays() []string
}

type OTSHandler interface {
	GetBitcoinRPC() *BTCRPCClient
	ConfigureBitcoinRPC(string, string, string) error
	UnsetBitcoinRPC()
}
