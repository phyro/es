(e)vent (s)tream
=====

_NOTE: This is a fork of [noscl](https://github.com/fiatjaf/noscl)._

A minimal verifiable event stream client for [Nostr](https://github.com/fiatjaf/nostr).

For every event, observed or created, it verifies the following:
- [x] Event signature
- [x] Event linearity (hashchain)
- [x] Event "ots" field provided


## Motivation

When we use voice to communicate, we encode information in a linear sequence. Voice is both authenticated (we hear/see who speaks) and linear (we hear the order of the words). Nostr events are authenticated with a digital signature, but they require trust about their order. This is because the owner can create an event with a timestamp (created_at) in the past or in the future and thus forge the event stream.

The idea is to bring linearity to Nostr events by treating events as an append-only event stream. Why should we worry about ordering at all?

Order verification is, like signature verification, done on the client side. An ordered event stream achieves two following properties:
- **Unforgeable history** - Following a user ensures they can't silently inject an event in the past. This holds true even for an attacker that steals your private key. Today, an attacker could create past events and try to forge history. With unforgeable history, the attacker can only append events and now the owner can append an event on the stream that signals key revocation. Any and all events identified on top of this chain can be ignored.
- **Missing event detection** - Fetching data for someone we follow could leave us with some missing events. This is impossible to detect today, but is easy to detect if the event stream forms a hashchain. This can help detect relays censoring certain events (i.e. a tweet) by serving the event only to users from some geopolitical region or decreasing the visibility rate of the event by serving it to every 10th user. Detection of missing events could make the follower query other relays to find the missind pieces or contact the user to republish their event chain.


Since we have no global linear event sequence like a blockchain does, we can't really agree on the order of events, but it's possible to find conflicts in the user event chain and thus prove they were not honestly publishing their events.

## How

Each event we create has an additional tag `{"prev": "<previous_event_id>"}`. This way the clients can verify they're building the same chain. The first event is an exeption and has prev set to "GENESIS". Verifying the hashchain puts events in order, but it tells very little about the time they were created at because the `created_at` can still be manipulated. To solve this, we use "ots" from [NIP-03](https://github.com/nostr-protocol/nips/blob/master/03.md) which uses OpenTimestamps attestations for our events. This makes sure that our events come with a very strong proof of their actual timestamp.

Following a user now simply means following their event stream. We ignore any and all events that don't build on top of it. We save the history of every event stream we own or follow in case we'd want to push this on other relays.


## Installation

Compile with `go install github.com/phyro/es@latest`.

## Usage

```
es

Usage:
  es home [--verbose]
  es create <name> <key>
  es create <name> [--gen]
  es remove <name>
  es switch <name>
  es ll [-a]
  es append <content>
  es follow <name> <pubkey>
  es unfollow <name>
  es sync [-a]
  es log [--name=<name>]
  es show <id> [--verbose]
  es relay
  es relay add <url>
  es relay remove <url>
```

The basic flow is something like

## Add some relays

```
$ es relay add wss://nostr-2.zebedee.cloud
```

## Generate an event stream

An event stream is a linear sequence of events. We can create a new one with

```
$ es create alice --gen
alice (4945495bd1f52d67b48b8a8a0ec4157b5a742b3ba210b4a30cc61bb3ef97d060)

Seed: illegal subway say over clean uphold liquid acid tired tilt reunion expect hand harsh ritual stock breeze pulse cattle tobacco galaxy surge peanut phone 
Private key: 37391bfacaa25ee6c4dce8328cc3a87d272a87842da43987c8b17bf138593660
```

## Set one event stream to active

We need to set one of the event streams to active to be able to display or append to them. We can switch between the streams with

```
$ es switch alice
```

## Display event streams

Event streams are either streams we own the private key for, or streams we're following. We can list the former with

```
$ es ll
bob (5c7b2a3a0151a3a304aa2789fa66196bf0adc394be5d9828529ae878697946c6)
* alice (ea1ef60519f531b5296c5fa14459a83faf3079b5ab2ec018d35c9d73f971fe29)
```

We can see that alice is marked with `* ` which means this is the currently active event stream.

We can also display all the other event streams (i.e. the ones we follow) with
```
$ es ll -a

bob (5c7b2a3a0151a3a304aa2789fa66196bf0adc394be5d9828529ae878697946c6)
* alice (ea1ef60519f531b5296c5fa14459a83faf3079b5ab2ec018d35c9d73f971fe29)

------------------------------------
Following:
------------------------------------
bob (5c7b2a3a0151a3a304aa2789fa66196bf0adc394be5d9828529ae878697946c6)
alice (ea1ef60519f531b5296c5fa14459a83faf3079b5ab2ec018d35c9d73f971fe29)
eve_ (fae519376ad3ea4274cc258c45abfcae1f679b9189d1443ea1cec3358cd0cf04)
```

We can see we follow also our own event streams. This is mostly to keep things simple, but it also helps with cross-device chain sync.

## Using streams

#### Append

We can append to the currently active event stream with
```
$ es append "Hi!"
```

This will send a new event to our relays as well as add the event to our local stream copy.

#### Follow

To follow an event stream we simply choose a name for it and run
```
$ es follow eve fae519376ad3ea4274cc258c45abfcae1f679b9189d1443ea1cec3358cd0cf04
eve (fae519376ad3ea4274cc258c45abfcae1f679b9189d1443ea1cec3358cd0cf04)
Followed fae519376ad3ea4274cc258c45abfcae1f679b9189d1443ea1cec3358cd0cf04.
Syncing eve ... Done
HEAD (eve) at: dac20073d1c657fd2268a3055f60fd226db76c991a7bf4122eff1a055775128b
```

This adds an event stream to our list and syncs its hashchain.

#### Unfollow

```
$ es unfollow eve
```

#### Sync stream

To get up to date event stream hashchain of the currently active stream run
```
$ es sync
Syncing alice ... Done
HEAD (alice) at: 29f55d3e4eee8ee516935bf7e5c36f006756d3b94e665dfdc326c6dbfe863dd0
```

To sync other streams add `--name=alice` flag.

#### Push stream

We can push the stream we hold locally to our relays with
```
$ es push bob
Pushing stream labeled as bob
Sent event 6393fc4a54e49d4d6ce44a59e2d864e59f2c2862510a5e4e2f99c71232b0358a to 'wss://nostr-2.zebedee.cloud'.
Sent event 52ba103a43528cf103ba301894587555d9fc2d9523eaf01fb7b5217164fdeb66 to 'wss://nostr-2.zebedee.cloud'.
Sent event 199ebf8af64e8ad7a621f685ceedffb5977dea770f2018cbdec6f9d93ac5c0c2 to 'wss://nostr-2.zebedee.cloud'.
Sent event 4e824123246daf8364ec21b9093d6267815a95cfb8ecbfebc4df6a36c7b9c61d to 'wss://nostr-2.zebedee.cloud'.
Sent event 7fc22ad63d049a80ee1839499c7788abcbd594ffd6ff3828a0026bb3dd01988f to 'wss://nostr-2.zebedee.cloud'.
Stream succesfully pushed.
```

#### Log

We can view the hashchain of the event stream with
```
$ es log
alice (ea1ef60519f531b5296c5fa14459a83faf3079b5ab2ec018d35c9d73f971fe29)

Event stream:
			NULL
			|
			v
9580244c7cd5a29d9c988a48d8a0968936c33c838355020d9f0831e2079138bb
			|
			v
15fcf8d7bbb639cf318704e8a2d7be5bd96f5f10d735834d383e93e213920fde
			|
			v
af9ea5419a85233df3ee327ee282092b34760d3c14494058cfd7bad377d6b698
			|
			v
7b32b990e140a77ed2405abb95427e6b2c0de2858f041a9bae55af87863b8ca9
			|
			v
6929908abab77da3017bf6c8c2fb05ac0625b2fcd5f16146eb5f7abe167793e6
			|
			v
29f55d3e4eee8ee516935bf7e5c36f006756d3b94e665dfdc326c6dbfe863dd0
```

Note that this is a view of our local stream copy, it doesn't fetch the chain from relays. Similarly like with sync, we can see a log of any local event stream by using the flag `--name=eve`.

#### OTS (OpenTimestamps)

We stamp every event with [OpenTimestamps](https://opentimestamps.org/) by implementing [NIP-03](https://github.com/nostr-protocol/nips/blob/master/03.md). We also require every event to come with the "ots" field. This field can only be verified by validating the proof against the Bitcoin blockchain. To verify them, we first have to configure the connection to our bitcoin rpc. We do this with

```
$ es ots rpc localhost:8332 myuser mysupersecretpassword
Bitcoin node version: 1
Successfully configured Bitcoin RPC.
```

We can now verify the stamps of a stream with
```
$ es ots verify bob

Event id: 6393fc4a54e49d4d6ce44a59e2d864e59f2c2862510a5e4e2f99c71232b0358a: Status: OK (2022-12-28 20:30:55 +0000 UTC)
Event id: 52ba103a43528cf103ba301894587555d9fc2d9523eaf01fb7b5217164fdeb66: Status: OK (2022-12-28 20:30:55 +0000 UTC)
Event id: 199ebf8af64e8ad7a621f685ceedffb5977dea770f2018cbdec6f9d93ac5c0c2: Status: OK (2022-12-28 20:30:55 +0000 UTC)
Event id: 4e824123246daf8364ec21b9093d6267815a95cfb8ecbfebc4df6a36c7b9c61d: Status: OK (2022-12-28 23:16:55 +0000 UTC)
Event id: 7fc22ad63d049a80ee1839499c7788abcbd594ffd6ff3828a0026bb3dd01988f: Status: OK (2022-12-29 15:36:02 +0000 UTC)
Event id: 21c21487282fd7c72cf8a95396dfaec82fdb75433c6cc7b3e95ff7d46603cd6f: Status: PENDING
```

We can see the last event is pending. OpenTimestamps can take a few hours to get our proof on the Bitcoin blockchain. But if we try this tomorrow, it should validate.
