package main

import (
	"bytes"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/nbd-wtf/go-nostr"
	"github.com/phyro/go-opentimestamps/opentimestamps"
	"github.com/phyro/go-opentimestamps/opentimestamps/client"
)

var (
	ErrOTSPending              = errors.New("pending")
	ErrOTSWaitingConfirmations = errors.New("waiting for 5 confirmations")
)

const defaultCalendar = "https://alice.btc.calendar.opentimestamps.org"

// The response of blockchain.info request
type BlockchainInfoResp struct {
	Blocks []BlocksResp `json:"blocks"`
}
type BlocksResp struct {
	MerkleRoot string `json:"mrkl_root"`
	Timestamp  int    `json:"time"`
}

type OTSService struct {
	rpcclient *BTCRPCClient
}

// OpenTimestamps an event and return the stamp data
func (o *OTSService) Stamp(ev *nostr.Event) (string, error) {
	cal, err := opentimestamps.NewRemoteCalendar(defaultCalendar)
	if err != nil {
		return "", fmt.Errorf("error creating remote calendar: %v", err)
	}
	digest_32 := sha256.Sum256(ev.Serialize())
	digest := digest_32[:]
	// base_path, _ := homedir.Expand(CONFIG_BASE_DIR)
	// // We will save the .ots in <event_id>.ots
	// path := filepath.Join(base_path, ev.ID)
	// outFile, err := os.Create(path + ".ots")
	// if err != nil {
	// 	return "", fmt.Errorf("error creating output file: %v", err)
	// }

	// Create a timestamp
	dts, err := opentimestamps.CreateDetachedTimestampForHash(digest, cal)
	if err != nil {
		return "", fmt.Errorf("error creating detached timestamp: %v", err)
	}
	// if err := dts.WriteToStream(outFile); err != nil {
	// 	return "", fmt.Errorf("error writing detached timestamp: %v", err)
	// }
	buf := new(bytes.Buffer)
	if err := dts.WriteToStream(buf); err != nil {
		return "", fmt.Errorf("error writing detached timestamp to buf: %v", err)
	}

	ots_content := buf.String()
	ots_b64 := b64.StdEncoding.EncodeToString([]byte(ots_content))
	return ots_b64, nil
}

// Poor man's implementation of a check if ots has been upgraded
func (o *OTSService) IsUpgraded(ev *nostr.Event) bool {
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

func (o *OTSService) Upgrade(ev *nostr.Event) (*opentimestamps.Timestamp, error) {
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

func (o *OTSService) Verify(ev *nostr.Event) (bool, *time.Time, error) {
	// TODO: When upgrading .ots, save it to prevent fetching it every time.
	// This might require updating the go-opentimestamps lib
	upgraded, err := o.Upgrade(ev)
	if err != nil {
		if err == ErrOTSPending {
			return true, nil, err
		}
		if err == ErrOTSWaitingConfirmations {
			return true, nil, err
		}
		return false, nil, err
	}

	if o.rpcclient != nil {
		return o.verifyRPC(upgraded)
	}
	// We have no bitcoin RPC client set. Query blockchain.info and output merkle root hashes
	// for manual verification in case the site isn't trusted
	return o.verifyManual(upgraded)
}

func (o *OTSService) verifyRPC(upgraded *opentimestamps.Timestamp) (bool, *time.Time, error) {
	if o.rpcclient == nil {
		return false, nil, errors.New("Trying to verify OTS with RPC without RPC client set")
	}
	btcConn, err := newBtcConn(o.rpcclient.Host, o.rpcclient.User, o.rpcclient.Password)
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

// Make a GET request on blockchain.info to fetch the merkle root and verifies the expected merkle root against that
func (o *OTSService) verifyManual(upgraded *opentimestamps.Timestamp) (bool, *time.Time, error) {
	verifier := client.NewBitcoinAttestationVerifier(nil)
	atts, err := verifier.VerifyManual(upgraded)
	if err != nil {
		return false, nil, fmt.Errorf("error verifying timestamp: %v", err)
	}

	// Make get requests on blockchain.info to verify against the merkle root
	base_url := "https://blockchain.info/block-height/"
	for _, att := range atts {
		expected_mekle_root := b2lx(att.ExpectedMerkleRoot)
		fmt.Printf("\nChecking if block at height: %d has merkle root: %s", att.Height, expected_mekle_root)

		url := fmt.Sprintf("%s%d%s", base_url, att.Height, "?format=json")
		res, err := http.Get(url)
		if err != nil {
			return false, nil, err
		}
		defer res.Body.Close()
		body, readErr := ioutil.ReadAll(res.Body)
		if readErr != nil {
			log.Fatal(readErr)
		}

		// Read and compare merkle root
		var result BlockchainInfoResp
		json.Unmarshal(body, &result)
		binfo_merkle_root := result.Blocks[0].MerkleRoot
		if expected_mekle_root == binfo_merkle_root {
			// Parse block timestamp from blockchain.info json
			ts := time.Unix(int64(result.Blocks[0].Timestamp), 0).UTC()
			return true, &ts, nil
		} else {
			return false, nil, fmt.Errorf("merkle root mismatch. Expected: %s, got: %s", expected_mekle_root, binfo_merkle_root)
		}
	}

	return true, nil, nil
}

func (o *OTSService) HasRPCConfigured() bool {
	return o.rpcclient != nil
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

// Convert to little endian hex
func b2lx(b []byte) string {
	// Reverse the slice
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	// Encode the reversed slice as a hex string
	return hex.EncodeToString(b)
}
