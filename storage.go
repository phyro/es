package main

// A backend interface for saving/reading streams & relays
type StorageBackend interface {
	// Event stream management
	CreateEventStream(string, string, bool)
	RemoveEventStream(string)
	SetActiveEventStream(string) error
	GetActiveStream() (*EventStream, error)

	// Core
	GetEventStream(string) (*EventStream, error)
	GetAllEventStreams() ([]*EventStream, error)
	SaveEventStream(*EventStream) error
	FollowEventStream(*Nostr, string, string, *BTCRPCClient) error
	UnfollowEventStream(string)

	// Misc
	ListEventStreams(bool) error
	GetPubForName(string) (string, error)

	// Relay
	AddRelay(string)
	RemoveRelay(string)
	ListRelays() []string

	// OTS RPC
	GetBitcoinRPC() *BTCRPCClient
	ConfigureBitcoinRPC(string, string, string) error
	UnsetBitcoinRPC()
}

// Identity for type checking atm
func NewStreamStore(s StorageBackend) StorageBackend {
	return s
}
