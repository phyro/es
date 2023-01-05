# TODO

- printOTSResult should be on OTSService and the result should be a struct with fields like status etc.
- add per stream relay pooling
- encrypt private keys and ask for a password for `append, follow, unfollow` actions
- potentially encrypt all the streams requiring a password for any action
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

- load balance requests to relays. Since we have a linear chain, we no longer need to fetch the same data from every relay. We simply fetch data from the first one, build the chain forward and ask the next relay to continue from our new head until we are no longer extending the chain. If we have ordered events, it's redundant to ask multiple relays for the same data because we can verify there are no missing parts.
- create a local MMR from the events. This way, we could construct an inclusion proof for any event.
- handle derived streams i.e. `es create facebook --from="alice"` which derives the private key from alice's private key with `H(alice_priv || "damus")`. This way we can create many separate event streams while only needing to save a single password.
