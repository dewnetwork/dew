package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// Anvil / Hardhat account #0 — well-known Ethereum test vector.
const (
	testPrivHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	testAddrHex = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
)

func TestGenerateKey_ProducesValidSecp256k1Pair(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if key == nil || key.D == nil {
		t.Fatal("expected non-nil private key")
	}
	priv := FromECDSA(key)
	if len(priv) != 32 {
		t.Fatalf("private key length = %d, want 32", len(priv))
	}
	pub := FromECDSAPub(&key.PublicKey)
	if len(pub) != 65 || pub[0] != 0x04 {
		t.Fatalf("uncompressed public key = %x (len %d), want 65 bytes starting with 0x04", pub, len(pub))
	}
}

func TestPubkeyToAddress_EthereumCompatible(t *testing.T) {
	privBytes, err := hex.DecodeString(testPrivHex)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ToECDSA(privBytes)
	if err != nil {
		t.Fatalf("ToECDSA: %v", err)
	}

	addr := PubkeyToAddress(&key.PublicKey)
	got := addr.Hex()
	if got != testAddrHex {
		t.Fatalf("address = %s, want %s", got, testAddrHex)
	}
	if len(addr.Bytes()) != 20 {
		t.Fatalf("address length = %d, want 20", len(addr.Bytes()))
	}
}

func TestSignAndVerify_Digest(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	msg := []byte("dew phase A1")
	digest := Keccak256(msg)
	if len(digest) != 32 {
		t.Fatalf("digest length = %d, want 32", len(digest))
	}

	sig, err := Sign(digest, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) != 65 {
		t.Fatalf("signature length = %d, want 65", len(sig))
	}

	pub, err := Ecrecover(digest, sig)
	if err != nil {
		t.Fatalf("Ecrecover: %v", err)
	}
	wantPub := FromECDSAPub(&key.PublicKey)
	if !bytes.Equal(pub, wantPub) {
		t.Fatalf("recovered pub = %x, want %x", pub, wantPub)
	}

	if !VerifySignature(wantPub, digest, sig[:64]) {
		t.Fatal("VerifySignature returned false for valid signature")
	}

	// Tampered digest must fail recovery match.
	badDigest := Keccak256([]byte("tampered"))
	badPub, err := Ecrecover(badDigest, sig)
	if err == nil && bytes.Equal(badPub, wantPub) {
		t.Fatal("expected recovery mismatch for tampered digest")
	}
}

func TestKeccak256_EmptyInput(t *testing.T) {
	// Keccak-256("") = c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
	want, _ := hex.DecodeString("c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470")
	got := Keccak256(nil)
	if !bytes.Equal(got, want) {
		t.Fatalf("Keccak256(nil) = %x, want %x", got, want)
	}
}

func TestAddress_HexAndBytesRoundTrip(t *testing.T) {
	addr, err := HexToAddress(testAddrHex)
	if err != nil {
		t.Fatalf("HexToAddress: %v", err)
	}
	if addr.Hex() != testAddrHex {
		// EIP-55 checksum should match the canonical test vector casing.
		t.Fatalf("Hex() = %s, want %s", addr.Hex(), testAddrHex)
	}
	_, err = HexToAddress("not-an-address")
	if err == nil {
		t.Fatal("expected error for invalid hex address")
	}
}
