// Command catalog-manifest regenerates manifest.json for the embedded
// builtin indexer catalog. Run from the backend/ directory:
//
//   go run ./cmd/catalog-manifest -src internal/catalog/builtin/definitions \
//                                  -out internal/catalog/builtin/manifest.json
//
// SHA-256 is computed for each .yml file and recorded alongside its
// version (read from a `version:` field in the YAML itself). The
// generated manifest is committed alongside the YAMLs.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type definition struct {
	ID      string `yaml:"id"`
	Version string `yaml:"version"`
}

type manifestEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
}

type manifest struct {
	Schema      int             `json:"schema"`
	Version     string          `json:"version"`
	GeneratedAt string          `json:"generatedAt"`
	Definitions []manifestEntry `json:"definitions"`
	Signature   []byte          `json:"signature,omitempty"`
}

// SigningBytes returns the canonical byte sequence to be signed: the
// manifest JSON without the Signature field. Mirrors builtin.Manifest.
func (m *manifest) SigningBytes() ([]byte, error) {
	tmp := struct {
		Schema      int             `json:"schema"`
		Version     string          `json:"version"`
		GeneratedAt string          `json:"generatedAt"`
		Definitions []manifestEntry `json:"definitions"`
	}{m.Schema, m.Version, m.GeneratedAt, m.Definitions}
	return json.Marshal(tmp)
}

// signManifest produces an Ed25519 signature over msg using key.
// Panics on invalid key length to match the catalog.Sign contract.
func signManifest(msg []byte, key ed25519.PrivateKey) []byte {
	if len(key) != ed25519.PrivateKeySize {
		log.Fatalf("--sign: invalid EASYSEARCH_CATALOG_PRIVKEY length %d", len(key))
	}
	return ed25519.Sign(key, msg)
}

func main() {
	src := flag.String("src", "internal/catalog/builtin/definitions", "definitions directory")
	out := flag.String("out", "internal/catalog/builtin/manifest.json", "manifest output path")
	schemaVersion := flag.String("schema-version", "2026.07.1", "manifest version tag")
	var sign bool
	flag.BoolVar(&sign, "sign", false, "sign manifest with $EASYSEARCH_CATALOG_PRIVKEY (base64, 64 bytes)")
	flag.Parse()

	files, err := filepath.Glob(filepath.Join(*src, "*.yml"))
	if err != nil {
		die("glob: %v", err)
	}
	sort.Strings(files)

	m := manifest{
		Schema:      1,
		Version:     *schemaVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Definitions: nil,
	}

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			die("read %s: %v", f, err)
		}
		var d definition
		if err := yaml.Unmarshal(raw, &d); err != nil {
			die("parse %s: %v", f, err)
		}
		if d.ID == "" || d.Version == "" {
			die("definition %s missing id or version", f)
		}
		sum := sha256.Sum256(raw)
		m.Definitions = append(m.Definitions, manifestEntry{
			ID:      d.ID,
			Version: d.Version,
			File:    "definitions/" + filepath.Base(f),
			SHA256:  hex.EncodeToString(sum[:]),
		})
	}

	enc, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		die("marshal: %v", err)
	}
	if sign {
		keyB64 := os.Getenv("EASYSEARCH_CATALOG_PRIVKEY")
		key, err := base64.StdEncoding.DecodeString(keyB64)
		if err != nil || len(key) != ed25519.PrivateKeySize {
			log.Fatalf("--sign: invalid EASYSEARCH_CATALOG_PRIVKEY: %v", err)
		}
		signing, err := m.SigningBytes()
		if err != nil {
			log.Fatalf("signing bytes: %v", err)
		}
		m.Signature = signManifest(signing, ed25519.PrivateKey(key))
		final, _ := json.MarshalIndent(&m, "", "  ")
		if err := os.WriteFile(*out, append(final, '\n'), 0o644); err != nil {
			log.Fatalf("rewrite: %v", err)
		}
	} else if err := os.WriteFile(*out, append(enc, '\n'), 0o644); err != nil {
		die("write %s: %v", *out, err)
	}
	fmt.Printf("wrote %s with %d definitions (version %s)\n", *out, len(m.Definitions), m.Version)
	_ = strings.TrimSpace
}

func die(format string, a ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, a...))
	os.Exit(1)
}