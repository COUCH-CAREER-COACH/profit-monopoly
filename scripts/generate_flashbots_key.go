package main

import (
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// Generate new ECDSA key pair for Flashbots authentication
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal("Failed to generate key:", err)
	}

	// Get the private key in hex format
	privateKeyHex := fmt.Sprintf("%x", crypto.FromECDSA(privateKey))
	fmt.Printf("Private Key: 0x%s\n", privateKeyHex)

	// Get the public address
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	fmt.Printf("Public Address: %s\n", address.Hex())
}
