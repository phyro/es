package main

import (
	"encoding/hex"
	"log"

	"github.com/btcsuite/btcd/btcec"

	"github.com/nbd-wtf/go-nostr/nip06"
)

func getPubKey(privateKey string) string {
	if keyb, err := hex.DecodeString(privateKey); err != nil {
		log.Printf("Error decoding key from hex: %s\n", err.Error())
		return ""
	} else {
		_, pubkey := btcec.PrivKeyFromBytes(btcec.S256(), keyb)
		return hex.EncodeToString(pubkey.X.Bytes())
	}
}

func keyGen() (string, string, error) {
	seedWords, err := nip06.GenerateSeedWords()
	if err != nil {
		log.Println(err)
		return "", "", err
	}

	seed := nip06.SeedFromWords(seedWords)

	sk, err := nip06.PrivateKeyFromSeed(seed)
	if err != nil {
		log.Println(err)
		return "", "", err
	}

	return seedWords, sk, nil
}
