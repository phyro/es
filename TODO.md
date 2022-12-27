# TODO

- make sure it works with multiple relays
- rename stream.go to core.go because syncing/appending/following event streams is the core functionality
- load identity map (pubkey -> name) and use `<name> (<pubkey>)` throughout the app
- OTS attestation for every event
- test properties with multipass
- bubbletea TUI

# Maybe

- handle derived streams i.e. `es create facebook --from="alice"` which derives the private key from alice's private key with `H(alice_priv || "damus")`. This way we can create many separate event streams while only needing to save a single password.
