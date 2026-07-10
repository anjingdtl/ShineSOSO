package catalog

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestSignAndVerifyRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil { t.Fatalf("keygen: %v", err) }
	body := []byte(`{"schema":1,"version":1,"definitions":[]}`)
	sig := Sign(body, priv)
	if !Verify(body, sig, pub) { t.Fatal("Verify should succeed for signed body") }
	if Verify([]byte(`{"schema":1,"version":2,"definitions":[]}`), sig, pub) {
		t.Fatal("Verify should fail when body is tampered")
	}
	if Verify(body, []byte{0x00, 0x01}, pub) {
		t.Fatal("Verify should reject malformed signature")
	}
}

func TestVerifyRejectsForeignKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	body := []byte(`{"k":1}`)
	sig := Sign(body, priv)
	if Verify(body, sig, otherPub) {
		t.Fatal("Verify must reject signature made by a different key")
	}
}
