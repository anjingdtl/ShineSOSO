package catalog

import "crypto/ed25519"

func Sign(body []byte, key ed25519.PrivateKey) []byte {
	if len(key) != ed25519.PrivateKeySize { panic("catalog.Sign: invalid private key size") }
	return ed25519.Sign(key, body)
}

func Verify(body, sig []byte, pub ed25519.PublicKey) bool {
	if len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize { return false }
	return ed25519.Verify(pub, body, sig)
}
