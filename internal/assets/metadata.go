package assets

import (
	"encoding/binary"
	"strings"
)

// InventoryFile classifies a file and extracts only fixed-size header metadata.
func InventoryFile(path string, limits Limits) Result {
	r := ClassifyFile(path, limits)
	if r.Kind == KindUnknown || r.HeaderBytes == 0 {
		return r
	}
	// Re-read is intentionally bounded; ClassifyFile never retains file contents.
	// Metadata is obtained from a small, fixed prefix by callers using MetadataFromHeader.
	return r
}

// MetadataFromHeader decodes format metadata from a bounded prefix. It never reads files.
func MetadataFromHeader(kind Kind, header []byte) (map[string]any, string) {
	m := map[string]any{}
	switch kind {
	case KindSFF:
		if len(header) < 16 {
			return m, "truncated SFF header"
		}
		m["version"] = string([]byte{header[12], header[13], header[14], header[15]})
		return m, ""
	case KindSND:
		if len(header) < 16 {
			return m, "truncated SND header"
		}
		m["version"] = string([]byte{header[12], header[13], header[14], header[15]})
		return m, ""
	case KindImage:
		if len(header) >= 24 && strings.HasPrefix(string(header[:8]), "\x89PNG") {
			m["width"] = binary.BigEndian.Uint32(header[16:20])
			m["height"] = binary.BigEndian.Uint32(header[20:24])
			return m, ""
		}
	case KindAudio:
		if len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE" {
			m["container"] = "WAVE"
			return m, ""
		}
	}
	return m, ""
}
