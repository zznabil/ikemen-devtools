// Package release builds deterministic, signed release metadata.
package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const Module = "github.com/ikemen-engine/ikemen-devtools"

var ErrMissingTimestamp = errors.New("explicit build timestamp is required")

type FileHash struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"publicKey"`
	Value     string `json:"value"`
}

type Metadata struct {
	Module         string     `json:"module"`
	Version        string     `json:"version"`
	Contract       string     `json:"contract"`
	BuildTimestamp string     `json:"buildTimestamp"`
	Tool           string     `json:"tool,omitempty"`
	Compiler       string     `json:"compiler,omitempty"`
	GoVersion      string     `json:"goVersion,omitempty"`
	OS             string     `json:"os,omitempty"`
	Arch           string     `json:"arch,omitempty"`
	VCSRevision    string     `json:"vcsRevision,omitempty"`
	VCSModified    string     `json:"vcsModified,omitempty"`
	VCSTime        string     `json:"vcsTime,omitempty"`
	Files          []FileHash `json:"files"`
	Signature      *Signature `json:"signature,omitempty"`
}

// Build hashes files in canonical path order. Timestamp must be supplied by the caller.
func Build(module, version, contract, timestamp string, paths []string) (Metadata, error) {
	if strings.TrimSpace(timestamp) == "" {
		return Metadata{}, ErrMissingTimestamp
	}
	if _, err := time.Parse(time.RFC3339, timestamp); err != nil {
		return Metadata{}, fmt.Errorf("invalid build timestamp: %w", err)
	}
	if strings.TrimSpace(module) == "" || strings.TrimSpace(version) == "" || strings.TrimSpace(contract) == "" {
		return Metadata{}, errors.New("module, version, and contract are required")
	}
	files := make([]FileHash, 0, len(paths))
	for _, path := range paths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
		if clean == "." || clean == "" {
			return Metadata{}, errors.New("file path is required")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return Metadata{}, fmt.Errorf("hash %q: %w", path, err)
		}
		digest := sha256.Sum256(data)
		files = append(files, FileHash{Path: clean, SHA256: hex.EncodeToString(digest[:])})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return Metadata{Module: module, Version: version, Contract: contract, BuildTimestamp: timestamp, Files: files}, nil
}

// CanonicalJSON returns stable compact JSON with lexicographically ordered object keys.
func CanonicalJSON(metadata Metadata) ([]byte, error) {
	if strings.TrimSpace(metadata.BuildTimestamp) == "" {
		return nil, ErrMissingTimestamp
	}
	files := append([]FileHash(nil), metadata.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if files == nil {
		files = []FileHash{}
	}
	payload := map[string]interface{}{
		"buildTimestamp": metadata.BuildTimestamp,
		"contract":       metadata.Contract,
		"files":          files,
		"module":         metadata.Module,
		"version":        metadata.Version,
	}
	if metadata.Tool != "" {
		payload["tool"] = metadata.Tool
	}
	if metadata.Compiler != "" {
		payload["compiler"] = metadata.Compiler
	}
	if metadata.GoVersion != "" {
		payload["goVersion"] = metadata.GoVersion
	}
	if metadata.OS != "" {
		payload["os"] = metadata.OS
	}
	if metadata.Arch != "" {
		payload["arch"] = metadata.Arch
	}
	if metadata.VCSRevision != "" {
		payload["vcsRevision"] = metadata.VCSRevision
	}
	if metadata.VCSModified != "" {
		payload["vcsModified"] = metadata.VCSModified
	}
	if metadata.VCSTime != "" {
		payload["vcsTime"] = metadata.VCSTime
	}
	if metadata.Signature != nil {
		payload["signature"] = metadata.Signature
	}
	return json.Marshal(payload)
}

func Sign(metadata Metadata, privateKey ed25519.PrivateKey) (Metadata, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Metadata{}, errors.New("invalid Ed25519 private key")
	}
	metadata.Signature = nil
	canonical, err := CanonicalJSON(metadata)
	if err != nil {
		return Metadata{}, err
	}
	signature := ed25519.Sign(privateKey, canonical)
	metadata.Signature = &Signature{Algorithm: "Ed25519", PublicKey: base64.RawStdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)), Value: base64.RawStdEncoding.EncodeToString(signature)}
	return metadata, nil
}

func Verify(metadata Metadata, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return errors.New("invalid Ed25519 public key")
	}
	if metadata.Signature == nil || metadata.Signature.Algorithm != "Ed25519" {
		return errors.New("missing or unsupported signature")
	}
	if metadata.Signature.PublicKey != base64.RawStdEncoding.EncodeToString(publicKey) {
		return errors.New("signature public key mismatch")
	}
	signature, err := base64.RawStdEncoding.DecodeString(metadata.Signature.Value)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid signature encoding")
	}
	metadata.Signature = nil
	canonical, err := CanonicalJSON(metadata)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, canonical, signature) {
		return errors.New("invalid signature")
	}
	return nil
}
