# TODO

- encrypt private keys and ask for a password for `append, follow, unfollow` actions
- potentially encrypt all the streams requiring a password for any action
- make stream.go functions use the storage backend interface type
- rename stream.go to core.go because syncing/appending/following event streams is the core functionality
- load identity map (pubkey -> name) and use `<name> (<pubkey>)` throughout the app
- test properties with multipass + multiple relays
- bubbletea TUI
- implement a proper backend storage i.e. sql or smth
- separate the core logic of event streams when the time is right
    - es-lib
    - es-cli
    - es-tui

### OTS

- save the upgraded version to avoid querying opentimestamp for an event multiple times
- verify the go-opentimestamps implementation (we should never say an event was attested at time T if it wasn't)
- make OTS more robust (more calendars)
- ideally hide the 'ots' commands from the user and do everything in the background

# Maybe

- handle derived streams i.e. `es create facebook --from="alice"` which derives the private key from alice's private key with `H(alice_priv || "damus")`. This way we can create many separate event streams while only needing to save a single password.
