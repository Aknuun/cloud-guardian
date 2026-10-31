package router

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestVerifyScriptSig(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	script := []byte("#!/usr/bin/env bash\necho hi\n")
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, script))
	hexKey := hex.EncodeToString(pub)

	if err := verifyScriptSig(script, sig, hexKey); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := verifyScriptSig([]byte("#!/bin/sh\nrm -rf /\n"), sig, hexKey); err == nil {
		t.Fatal("tampered script accepted")
	}
	if err := verifyScriptSig(script, "not-base64", hexKey); err == nil {
		t.Fatal("garbage signature accepted")
	}
}
