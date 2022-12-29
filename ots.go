package main

import (
	"bytes"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/mitchellh/go-homedir"
	"github.com/nbd-wtf/go-nostr"
	"github.com/phyro/go-opentimestamps/opentimestamps"
	"github.com/phyro/go-opentimestamps/opentimestamps/client"
)

// Status
var (
	ErrOTSPending              = errors.New("pending")
	ErrOTSWaitingConfirmations = errors.New("waiting for 5 confirmations")
)

const defaultCalendar = "https://alice.btc.calendar.opentimestamps.org"

func stamp(ev *nostr.Event) string {
	cal, err := opentimestamps.NewRemoteCalendar(defaultCalendar)
	if err != nil {
		log.Fatalf("error creating remote calendar: %v", err)
	}

	digest_32 := sha256.Sum256(ev.Serialize())
	digest := digest_32[:]

	base_path, _ := homedir.Expand(BASE_DIR)
	// We will save the .ots in <event_id>.ots
	path := filepath.Join(base_path, ev.ID)
	outFile, err := os.Create(path + ".ots")
	if err != nil {
		log.Fatalf("error creating output file: %v", err)
	}

	dts, err := opentimestamps.CreateDetachedTimestampForHash(digest, cal)
	if err != nil {
		log.Fatalf(
			"error creating detached timestamp for %s: %v",
			path, err,
		)
	}

	if err := dts.WriteToStream(outFile); err != nil {
		log.Fatalf("error writing detached timestamp: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := dts.WriteToStream(buf); err != nil {
		log.Fatalf("error writing detached timestamp to buf: %v", err)
	}
	return buf.String()
}

// Poor man's implementation of a check if ots has been upgraded
func is_ots_upgraded(ev *nostr.Event) bool {
	/*
		bitcoinAttestationTag = mustDecodeHex("0588960d73d71901")
		pendingAttestationTag = mustDecodeHex("83dfe30d2ef90c8e")
	*/
	bitcoinAttestationTag := "0588960d73d71901"
	pendingAttestationTag := "83dfe30d2ef90c8e"
	ots_b64 := ev.GetExtra("ots").(string)
	ots, err := b64.StdEncoding.DecodeString(ots_b64)
	if err != nil {
		log.Panic(err.Error())
	}
	hx := hex.EncodeToString([]byte(ots))
	found_bitcoin_tag := strings.Contains(hx, bitcoinAttestationTag)
	found_pending_tag := strings.Contains(hx, pendingAttestationTag)
	if found_bitcoin_tag && found_pending_tag {
		log.Panicf("\nFound both ots tags for event id: %s", ev.ID)
	}
	if found_bitcoin_tag {
		return true
	}
	if found_pending_tag {
		return false
	}
	// This means we didn't find any of the tags...
	log.Panicf("\nFound no ots tags for event id: %s", ev.ID)
	return false
}

func ots_upgrade(ev *nostr.Event) (*opentimestamps.Timestamp, error) {
	ots_b64 := ev.GetExtra("ots").(string)
	ots, err := b64.StdEncoding.DecodeString(ots_b64)
	if err != nil {
		log.Panic(err.Error())
	}
	ots_reader := bytes.NewReader(ots)
	dts, _ := opentimestamps.NewDetachedTimestampFromReader(ots_reader)

	var upgraded *opentimestamps.Timestamp

	for _, pts := range opentimestamps.PendingTimestamps(dts.Timestamp) {
		upgraded, err = pts.Upgrade()
		if err != nil {
			if strings.Contains(err.Error(), "Pending confirmation in Bitcoin blockchain") {
				return nil, ErrOTSPending
			} else if strings.Contains(err.Error(), "waiting for 5 confirmations") {
				return nil, ErrOTSWaitingConfirmations
			} else {
				return nil, err
			}
		}
		// FIXME merge timestamp instead of replacing it
		return upgraded, nil
	}

	return nil, fmt.Errorf("OTS upgrade did not happen")
}

func ots_verify(ev *nostr.Event, rpcclient BTCRPCClient) (bool, *time.Time, error) {
	// TODO: When upgrading .ots, save it to prevent fetching it every time.
	// This might require updating the go-opentimestamps lib
	upgraded, err := ots_upgrade(ev)
	if err != nil {
		if err == ErrOTSPending { // || err == ErrOTSWaitingConfirmations {
			return true, nil, err
		}
		// TODO: should we return "true" here if we get ErrOTSWaitingConfirmations?
		return false, nil, err
	}

	// btcConn, err := newBtcConn("localhost:8332", "oohuser", "oohpass")
	btcConn, err := newBtcConn(rpcclient.Host, rpcclient.User, rpcclient.Password)
	if err != nil {
		return false, nil, fmt.Errorf("error creating btc connection: %v", err)
	}

	verifier := client.NewBitcoinAttestationVerifier(btcConn)

	ts, err := verifier.Verify(upgraded)
	if err != nil {
		return false, nil, fmt.Errorf("error verifying timestamp: %v", err)
	}
	if ts == nil {
		return false, nil, fmt.Errorf("no bitcoin-verifiable timestamps found")
	}

	return true, ts, nil
}

func newBtcConn(host, user, pass string) (*rpcclient.Client, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         host,
		User:         user,
		Pass:         pass,
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	return rpcclient.New(connCfg, nil)
}
