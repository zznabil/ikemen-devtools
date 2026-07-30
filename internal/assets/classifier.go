// Package assets classifies workspace files and extracts bounded metadata.
package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
	"os"
	"path/filepath"
	"strings"
)

type Kind string

const (
	KindAuthoredText Kind = "authored-text"
	KindSFF          Kind = "sff"
	KindSND          Kind = "snd"
	KindFont         Kind = "font"
	KindShader       Kind = "shader"
	KindImage        Kind = "image"
	KindAudio        Kind = "audio"
	KindUnknown      Kind = "unknown"
)

type Limits struct{ MaxTextBytes, MaxHeaderBytes int64 }

func DefaultLimits() Limits { return Limits{MaxTextBytes: 8 << 20, MaxHeaderBytes: 64 << 10} }

type Result struct {
	Path          string          `json:"path"`
	Kind          Kind            `json:"kind"`
	MIME          string          `json:"mime"`
	Size          int64           `json:"size"`
	HeaderBytes   int             `json:"headerBytes"`
	Identity      string          `json:"identity,omitempty"`
	HashPolicy    string          `json:"hashPolicy,omitempty"`
	FormatVersion string          `json:"formatVersion,omitempty"`
	ParseComplete bool            `json:"parseComplete"`
	Truncated     bool            `json:"truncated"`
	Metadata      map[string]any  `json:"metadata,omitempty"`
	Diagnostics   []ir.Diagnostic `json:"diagnostics,omitempty"`
}

// ClassifyFile opens only a bounded prefix and never allocates based on file size.
func ClassifyFile(path string, limits Limits) Result {
	r := Result{Path: filepath.Clean(path), Kind: KindUnknown, MIME: "application/octet-stream", Metadata: map[string]any{}}
	if limits.MaxTextBytes <= 0 {
		limits.MaxTextBytes = DefaultLimits().MaxTextBytes
	}
	if limits.MaxHeaderBytes <= 0 {
		limits.MaxHeaderBytes = DefaultLimits().MaxHeaderBytes
	}
	st, err := os.Stat(path)
	if err != nil {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "asset-stat", err.Error()))
		return r
	}
	if !st.Mode().IsRegular() {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "asset-not-regular", "asset is not a regular file"))
		return r
	}
	r.Size = st.Size()
	f, err := os.Open(path)
	if err != nil {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "asset-open", err.Error()))
		return r
	}
	defer f.Close()
	n := limits.MaxHeaderBytes
	if n > limits.MaxTextBytes {
		n = limits.MaxTextBytes
	}
	if n > 0 && int64(int(n)) != n {
		n = int64(^uint(0) >> 1)
	}
	buf := make([]byte, int(n))
	got, readErr := f.Read(buf)
	buf = buf[:got]
	r.HeaderBytes = got
	identityInput := fmt.Sprintf("%s:%d:%d:%x", filepath.ToSlash(r.Path), r.Size, st.ModTime().UnixNano(), buf)
	digest := sha256.Sum256([]byte(identityInput))
	r.Identity = hex.EncodeToString(digest[:])
	r.HashPolicy = "path-size-mtime-header-sha256"
	if readErr != nil && readErr.Error() != "EOF" {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "asset-read", readErr.Error()))
	}
	r.Kind, r.MIME = classify(filepath.Ext(path), buf)
	metadata, metadataErr := MetadataFromHeader(r.Kind, buf)
	r.Metadata = metadata
	if metadataErr != "" {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "metadata-header", metadataErr))
	}
	if r.Kind == KindAuthoredText && r.Size > limits.MaxTextBytes {
		r.Truncated = true
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "text-limit", "authored text exceeds bounded read limit"))
	}
	if r.Kind == KindUnknown {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "unsupported-format", "file format is not recognized"))
	}
	if r.Kind == KindSFF && len(buf) < 16 {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "sff-header", "truncated SFF header"))
	}
	if r.Kind == KindSND && len(buf) < 16 {
		r.Diagnostics = append(r.Diagnostics, diagnostic(r.Path, "snd-header", "truncated SND header"))
	}
	r.ParseComplete = len(r.Diagnostics) == 0
	return r
}

func classify(ext string, b []byte) (Kind, string) {
	if len(b) >= 11 && bytes.Equal(b[:11], []byte("ElecbyteSpr")) {
		return KindSFF, "application/x-sff"
	}
	if len(b) >= 11 && bytes.Equal(b[:11], []byte("ElecbyteSnd")) {
		return KindSND, "audio/x-ikemen-snd"
	}
	if len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return KindImage, "image/png"
	}
	if len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WAVE")) {
		return KindAudio, "audio/wav"
	}
	if len(b) >= 4 && (bytes.Equal(b[:4], []byte("OTTO")) || bytes.Equal(b[:4], []byte("wOFF")) || bytes.Equal(b[:4], []byte("wOF2")) || binary.BigEndian.Uint32(b[:4]) == 0x00010000) {
		return KindFont, "font/otf"
	}
	if len(b) >= 4 && bytes.Equal(b[:4], []byte("OggS")) {
		return KindAudio, "audio/ogg"
	}
	if len(b) >= 3 && bytes.Equal(b[:3], []byte("ID3")) {
		return KindAudio, "audio/mpeg"
	}
	e := strings.ToLower(ext)
	if e == ".glsl" || e == ".vert" || e == ".frag" || e == ".shader" || e == ".hlsl" || e == ".fx" {
		return KindShader, "text/x-shader"
	}
	switch e {
	case ".def", ".cns", ".st", ".cmd", ".air", ".lua", ".zss", ".ini", ".cfg", ".txt", ".json", ".xml", ".yaml", ".yml", ".toml":
		return KindAuthoredText, "text/plain; charset=utf-8"
	}
	switch e {
	case ".png":
		return KindImage, "image/png"
	case ".jpg", ".jpeg":
		return KindImage, "image/jpeg"
	case ".bmp":
		return KindImage, "image/bmp"
	case ".gif":
		return KindImage, "image/gif"
	case ".ttf":
		return KindFont, "font/ttf"
	case ".otf":
		return KindFont, "font/otf"
	case ".wav":
		return KindAudio, "audio/wav"
	case ".ogg":
		return KindAudio, "audio/ogg"
	case ".mp3":
		return KindAudio, "audio/mpeg"
	}
	return KindUnknown, "application/octet-stream"
}

func diagnostic(path, code, message string) ir.Diagnostic {
	return ir.Diagnostic{Path: path, Code: code, Severity: ir.SeverityWarning, Message: message, Start: ir.SourcePosition{Line: 1, Column: 1}, End: ir.SourcePosition{Line: 1, Column: 1}}
}
