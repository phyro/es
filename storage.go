package main

// A backend interface for saving/reading streams & relays
type StorageBackend interface {
	// Event stream management
	CreateEventStream(string, string, bool)
	RemoveEventStream(string)
	SetActiveEventStream(string) error
	GetActiveStream() (EventStream, error)

	// Core
	GetEventStream(string) (EventStream, error)
	GetAllEventStreams() ([]EventStream, error)
	SaveEventStream(EventStream) error
	FollowEventStream(Nostr, string, string, *BTCRPCClient) error
	UnfollowEventStream(string)

	// Misc
	ListEventStreams(bool) error

	// Relay
	AddRelay(string)
	RemoveRelay(string)
	ListRelays()

	// OTS RPC
	ConfigureBitcoinRPC(string, string, string) error
}
