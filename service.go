package main

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
