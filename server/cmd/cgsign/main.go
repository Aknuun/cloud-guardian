// Signs a file with the release ed25519 key and prints base64 signature.
// Usage: (cd server && go run ./cmd/cgsign /root/cg-signing.key ../install-server.sh)
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: sign.go <key.pem> <file>")
		os.Exit(2)
	}
	kb, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	blk, _ := pem.Decode(kb)
	if blk == nil {
		fmt.Fprintln(os.Stderr, "key is not PEM")
		os.Exit(1)
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		fmt.Fprintln(os.Stderr, "key is not ed25519")
		os.Exit(1)
	}
	msg, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg)))
}
