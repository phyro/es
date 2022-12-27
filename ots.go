package main

import (
	"bytes"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/mitchellh/go-homedir"
	"github.com/nbd-wtf/go-nostr"
	"github.com/phyro/go-opentimestamps/opentimestamps"
	"github.com/phyro/go-opentimestamps/opentimestamps/client"
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

func is_ots_upgraded(ev *nostr.Event) bool {
	// A poor man's implementation of a check if ots has been upgraded
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
	// fmt.Printf("\nots content: %s\n", ots)
	// fmt.Printf("\nots hex content: %s\n", hx)
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

	/*

		ots_b64 := ev.GetExtra("ots").(string)
		ots, err := b64.StdEncoding.DecodeString(ots_b64)
		hx := hex.EncodeToString([]byte(ots))
		fmt.Printf("\nots content: %s\n", ots)
		fmt.Printf("\nots hex content: %s\n", hx)
		if err != nil {
			log.Panic(err.Error())
		}
		ots_reader := bytes.NewReader(ots)
		dts, err := opentimestamps.NewDetachedTimestampFromReader(ots_reader)
		if err != nil {
			log.Panic(err.Error())
		}
		// fmt.Printf("\nmsg: %s\n", dts.Timestamp.Message)
		// for _, att := range dts.Timestamp.Attestations {
		// 	fmt.Println("Found attestation!")
		// 	// Try casting attestation to btc attenstation (upgraded)
		// 	_, ok := att.(*opentimestamps.BitcoinAttestation)
		// 	if !ok {
		// 		continue
		// 	}
		// 	fmt.Printf("\nIs upgraded: %t\n", ok)
		// 	return ok
		// }

		ots_reader = bytes.NewReader(ots)
		var ts *opentimestamps.Timestamp
		ts, err = opentimestamps.NewTimestampFromReader(ots_reader, dts.Timestamp.Message)
		if err != nil {
			log.Panic(err.Error())
		} else {
			for _, att := range ts.Attestations {
				fmt.Println("Found attestation!")
				// Try casting attestation to btc attenstation (upgraded)
				_, btc_ok := att.(*opentimestamps.BitcoinAttestation)
				if !btc_ok {
					continue
				}
				fmt.Printf("\nIs upgraded: %t\n", btc_ok)
				return true
			}
		}

		return false
		// TODO: remove the loop and assert there's only 1 attestation for now

		// var dts *opentimestamps.Timestamp
		// dts, err = opentimestamps.NewTimestampFromReader(ots_reader)
	*/
}

func ots_upgrade(ev *nostr.Event) (string, error) {
	ots_b64 := ev.GetExtra("ots").(string)
	ots, err := b64.StdEncoding.DecodeString(ots_b64)
	// fmt.Printf("\nold_ots: %s\n", ots)
	if err != nil {
		log.Panic(err.Error())
	}
	ots_reader := bytes.NewReader(ots)
	dts, _ := opentimestamps.NewDetachedTimestampFromReader(ots_reader)

	var upgraded *opentimestamps.Timestamp

	for _, pts := range opentimestamps.PendingTimestamps(dts.Timestamp) {
		// fmt.Printf(
		// 	"#%2d: upgrade %v\n     %x\n    ",
		// 	n, pts.PendingAttestation, pts.Timestamp.Message,
		// )
		fmt.Printf("; STATUS: ")
		u, err := pts.Upgrade()
		if err != nil {
			if strings.Contains(err.Error(), "Pending confirmation in Bitcoin blockchain") {
				return "", fmt.Errorf("pending")
			} else if strings.Contains(err.Error(), "waiting for 5 confirmations") {
				return "", fmt.Errorf("waiting for 5 confirmations")
			} else {
				fmt.Printf("error %v", err)
				return "", err
			}
		} else {
			fmt.Printf("success")
		}
		// fmt.Print("\n")

		// FIXME merge timestamp instead of replacing it
		upgraded = u
		fmt.Printf("\nUpgraded msg: %s\n", hex.EncodeToString(upgraded.Message))
		break
	}

	if upgraded == nil {
		fmt.Printf("no pending timestamps found")
		return "", fmt.Errorf("OTS upgrade did not happen")
	}

	dts.Timestamp = upgraded
	// f, err := os.Create(path)
	// if err != nil {
	// 	log.Fatalf("error opening output file: %v", err)
	// }
	// defer f.Close()
	buf := new(bytes.Buffer)
	if err := dts.WriteToStream(buf); err != nil {
		log.Fatalf("error writing detached timestamp: %v", err)
	}
	// log.Print("timestamp updated successfully")
	return buf.String(), nil
}

func ots_upgrade_direct(ev *nostr.Event) (*opentimestamps.Timestamp, error) {
	ots_b64 := ev.GetExtra("ots").(string)
	ots, err := b64.StdEncoding.DecodeString(ots_b64)
	// fmt.Printf("\nold_ots: %s\n", ots)
	if err != nil {
		log.Panic(err.Error())
	}
	ots_reader := bytes.NewReader(ots)
	dts, _ := opentimestamps.NewDetachedTimestampFromReader(ots_reader)

	var upgraded *opentimestamps.Timestamp

	for _, pts := range opentimestamps.PendingTimestamps(dts.Timestamp) {
		// fmt.Printf(
		// 	"#%2d: upgrade %v\n     %x\n    ",
		// 	n, pts.PendingAttestation, pts.Timestamp.Message,
		// )
		fmt.Printf("; STATUS: ")
		u, err := pts.Upgrade()
		if err != nil {
			if strings.Contains(err.Error(), "Pending confirmation in Bitcoin blockchain") {
				return nil, fmt.Errorf("pending")
			} else if strings.Contains(err.Error(), "waiting for 5 confirmations") {
				return nil, fmt.Errorf("waiting for 5 confirmations")
			} else {
				fmt.Printf("error %v", err)
				return nil, err
			}
		} else {
			fmt.Printf("success")
		}
		// fmt.Print("\n")

		// FIXME merge timestamp instead of replacing it
		upgraded = u
		fmt.Printf("\nUpgraded msg: %s\n", hex.EncodeToString(upgraded.Message))
		return u, nil
	}

	return nil, fmt.Errorf("OTS upgrade did not happen")
}

func ots_verify_direct(ev *nostr.Event) (bool, error) {
	var err error
	upgraded, err := ots_upgrade_direct(ev)
	if err != nil {
		return false, err
	}

	// // Ask a node
	// ots_reader := bytes.NewReader([]byte(ots))
	// dts, err := opentimestamps.NewDetachedTimestampFromReader(ots_reader)
	// // dts, err := opentimestamps.NewDetachedTimestampFromPath(path)
	// if err != nil {
	// 	log.Panic(err.Error())
	// }

	// TMP
	// ots_reader2 := bytes.NewReader([]byte(ots))
	// var ts2 *opentimestamps.Timestamp
	// ts2, err = opentimestamps.NewTimestampFromReader(ots_reader2, dts.Timestamp.Message)
	// if err != nil {
	// 	log.Panic(err.Error())
	// } else {
	// 	fmt.Printf("\nmsg hex: %s", hex.EncodeToString(dts.Timestamp.Message))
	// 	for _, att := range ts2.Attestations {
	// 		fmt.Println("Found attestation!")
	// 		// Try casting attestation to btc attenstation (upgraded)
	// 		btcAtt, btc_ok := att.(*opentimestamps.BitcoinAttestation)
	// 		if !btc_ok {
	// 			fmt.Println("btc_ok false")
	// 			continue
	// 		} else {
	// 			fmt.Println("btc_ok true")
	// 		}
	// 		fmt.Printf("\nbtcAtt.Height: %d\n", btcAtt.Height)
	// 		// fmt.Printf("\nIs upgraded: %t\n", btc_ok)
	// 	}
	// }
	// ENDTMP

	btcConn, err := newBtcConn("localhost:8332", "oohuser", "oohpass")
	if err != nil {
		log.Fatalf("error creating btc connection: %v", err)
	}

	verifier := client.NewBitcoinAttestationVerifier(btcConn)

	ts, err := verifier.Verify(upgraded)
	if err != nil {
		log.Fatalf("error verifying timestamp: %v", err)
		return false, err
	}
	if ts == nil {
		fmt.Printf("no bitcoin-verifiable timestamps found\n")
		return false, nil
	}
	fmt.Printf("attested time: %v\n", ts)

	return true, nil
}

func ots_verify(ev *nostr.Event) (bool, error) {
	var err error
	ots := ""
	if !is_ots_upgraded(ev) {
		// TODO: should fail here if upgrade fails
		ots, err = ots_upgrade(ev)
		if err != nil {
			return false, err
		}
	} else {
		ots_b64 := ev.GetExtra("ots").(string)
		ots_b, err := b64.StdEncoding.DecodeString(ots_b64)
		if err != nil {
			log.Println(err.Error())
			return false, err
		}
		ots = string(ots_b[:])
	}

	// Ask a node
	ots_reader := bytes.NewReader([]byte(ots))
	dts, err := opentimestamps.NewDetachedTimestampFromReader(ots_reader)
	// dts, err := opentimestamps.NewDetachedTimestampFromPath(path)
	if err != nil {
		log.Panic(err.Error())
	}

	// TMP
	// ots_reader2 := bytes.NewReader([]byte(ots))
	// var ts2 *opentimestamps.Timestamp
	// ts2, err = opentimestamps.NewTimestampFromReader(ots_reader2, dts.Timestamp.Message)
	// if err != nil {
	// 	log.Panic(err.Error())
	// } else {
	// 	fmt.Printf("\nmsg hex: %s", hex.EncodeToString(dts.Timestamp.Message))
	// 	for _, att := range ts2.Attestations {
	// 		fmt.Println("Found attestation!")
	// 		// Try casting attestation to btc attenstation (upgraded)
	// 		btcAtt, btc_ok := att.(*opentimestamps.BitcoinAttestation)
	// 		if !btc_ok {
	// 			fmt.Println("btc_ok false")
	// 			continue
	// 		} else {
	// 			fmt.Println("btc_ok true")
	// 		}
	// 		fmt.Printf("\nbtcAtt.Height: %d\n", btcAtt.Height)
	// 		// fmt.Printf("\nIs upgraded: %t\n", btc_ok)
	// 	}
	// }
	// ENDTMP

	btcConn, err := newBtcConn("localhost:8332", "oohuser", "oohpass")
	if err != nil {
		log.Fatalf("error creating btc connection: %v", err)
	}

	verifier := client.NewBitcoinAttestationVerifier(btcConn)

	ts, err := verifier.Verify(dts.Timestamp)
	if err != nil {
		log.Fatalf("error verifying timestamp: %v", err)
		return false, err
	}
	if ts == nil {
		fmt.Printf("no bitcoin-verifiable timestamps found\n")
		return false, nil
	}
	fmt.Printf("attested time: %v\n", ts)

	return true, nil
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

var (
	flagBTCHost = flag.String("btc-host", "localhost:8332", "bitcoin-rpc hostname")
	flagBTCUser = flag.String("btc-user", "bitcoin", "bitcoin-rpc username")
	flagBTCPass = flag.String("btc-pass", "bitcoin", "bitcoin-rpc password")
)

// func b2lx(b []byte) string {
// 	// Reverse the slice
// 	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
// 		b[i], b[j] = b[j], b[i]
// 	}
// 	// Encode the reversed slice as a hex string
// 	return hex.EncodeToString(b)
// }
