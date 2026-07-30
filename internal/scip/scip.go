// Package scip exports the devtools IR as a small, deterministic SCIP-compatible
// JSON Lines representation. It intentionally uses only the Go standard library.
//
// Each line is a JSON object with a type field. The first line is metadata. Each
// following document line contains a canonical slash-separated path, language,
// symbols, occurrences, and diagnostics. Positions are zero-based lines and
// UTF-8 byte columns, matching SCIP's position convention. Occurrence roles are
// the strings "definition" and "reference". Symbol identities retain the IR
// identity contract fields so consumers can safely correlate records.
package scip

import (
	"bufio"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ikemen-engine/ikemen-devtools/internal/ir"
)

type metadata struct {
	Type                    string `json:"type"`
	Protocol                string `json:"protocol"`
	Tool                    string `json:"tool"`
	IdentityContractVersion string `json:"identityContractVersion"`
}
type identity struct {
	ContractVersion string `json:"contractVersion,omitempty"`
	SemanticKey     string `json:"semanticKey,omitempty"`
	OccurrenceID    string `json:"occurrence,omitempty"`
	StoreID         string `json:"store,omitempty"`
}
type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type sourceRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}
type symbolRecord struct {
	ID       string      `json:"id"`
	Identity identity    `json:"identity"`
	Name     string      `json:"name"`
	Kind     string      `json:"kind"`
	Range    sourceRange `json:"range"`
}
type occurrenceRecord struct {
	Symbol   string      `json:"symbol"`
	Identity identity    `json:"identity"`
	Range    sourceRange `json:"range"`
	Roles    []string    `json:"roles"`
	Target   string      `json:"target,omitempty"`
}
type diagnosticRecord struct {
	Range         sourceRange `json:"range"`
	Code          string      `json:"code,omitempty"`
	Severity      string      `json:"severity"`
	Message       string      `json:"message"`
	RelatedSymbol string      `json:"relatedSymbol,omitempty"`
}
type documentRecord struct {
	Type        string             `json:"type"`
	Path        string             `json:"path"`
	Language    string             `json:"language"`
	Symbols     []symbolRecord     `json:"symbols"`
	Occurrences []occurrenceRecord `json:"occurrences"`
	Diagnostics []diagnosticRecord `json:"diagnostics"`
}

// Export writes metadata and documents as newline-delimited JSON.
func Export(w io.Writer, documents []ir.Document) error {
	if w == nil {
		return io.ErrClosedPipe
	}
	bw := bufio.NewWriter(w)
	if err := writeJSON(bw, metadata{Type: "metadata", Protocol: "scip-jsonl-subset-1", Tool: "ikemen-devtools", IdentityContractVersion: ir.IdentityContractVersion}); err != nil {
		return err
	}
	for _, doc := range buildDocuments(documents) {
		if err := writeJSON(bw, doc); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// Marshal returns the exact bytes Export would write.
func Marshal(documents []ir.Document) ([]byte, error) {
	var b strings.Builder
	if err := Export(&b, documents); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func writeJSON(w *bufio.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err = w.Write(data); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

func buildDocuments(input []ir.Document) []documentRecord {
	docs := append([]ir.Document(nil), input...)
	sort.SliceStable(docs, func(i, j int) bool {
		left, right := canonicalPath(docs[i].Path), canonicalPath(docs[j].Path)
		if left != right {
			return left < right
		}
		if docs[i].FileType != docs[j].FileType {
			return docs[i].FileType < docs[j].FileType
		}
		return firstSymbolID(docs[i]) < firstSymbolID(docs[j])
	})
	out := make([]documentRecord, 0, len(docs))
	for _, doc := range docs {
		r := documentRecord{Type: "document", Path: canonicalPath(doc.Path), Language: doc.FileType, Symbols: []symbolRecord{}, Occurrences: []occurrenceRecord{}, Diagnostics: []diagnosticRecord{}}
		for _, sym := range doc.Symbols {
			id := symbolID(sym)
			r.Symbols = append(r.Symbols, symbolRecord{ID: id, Identity: toIdentity(sym.Identity), Name: sym.Name, Kind: string(sym.Kind), Range: toRange(sym.Span)})
			r.Occurrences = append(r.Occurrences, occurrenceRecord{Symbol: id, Identity: toIdentity(sym.Identity), Range: toRange(sym.Span), Roles: []string{"definition"}})
		}
		for _, ref := range doc.References {
			r.Occurrences = append(r.Occurrences, occurrenceRecord{Symbol: referenceSymbol(ref), Identity: toIdentity(ref.Identity), Range: toRange(ref.Span), Roles: []string{"reference"}, Target: ref.Target})
		}
		for _, d := range doc.Diagnostics {
			r.Diagnostics = append(r.Diagnostics, diagnosticRecord{Range: toRange(ir.SourceSpan{Start: d.Start, End: d.End}), Code: d.Code, Severity: string(d.Severity), Message: d.Message, RelatedSymbol: d.RelatedSymbol})
		}
		sort.SliceStable(r.Symbols, func(i, j int) bool { return r.Symbols[i].ID < r.Symbols[j].ID })
		sort.SliceStable(r.Occurrences, func(i, j int) bool { return occurrenceKey(r.Occurrences[i]) < occurrenceKey(r.Occurrences[j]) })
		sort.SliceStable(r.Diagnostics, func(i, j int) bool { return diagnosticKey(r.Diagnostics[i]) < diagnosticKey(r.Diagnostics[j]) })
		out = append(out, r)
	}
	return out
}

func canonicalPath(path string) string {
	p := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	return strings.TrimPrefix(p, "./")
}
func firstSymbolID(d ir.Document) string {
	if len(d.Symbols) == 0 {
		return ""
	}
	return symbolID(d.Symbols[0])
}
func symbolID(s ir.Symbol) string {
	if s.ID != "" {
		return s.ID
	}
	if s.Identity.StoreID != "" {
		return s.Identity.StoreID
	}
	if s.Identity.OccurrenceID != "" {
		return s.Identity.OccurrenceID
	}
	return s.Identity.SemanticKey
}
func referenceSymbol(r ir.Reference) string {
	if r.Target != "" {
		return r.Target
	}
	if r.SourceSymbol != "" {
		return r.SourceSymbol
	}
	if r.Identity.StoreID != "" {
		return r.Identity.StoreID
	}
	return r.Identity.OccurrenceID
}
func toIdentity(i ir.Identity) identity {
	if i.ContractVersion == "" {
		i.ContractVersion = ir.IdentityContractVersion
	}
	return identity{ContractVersion: i.ContractVersion, SemanticKey: i.SemanticKey, OccurrenceID: i.OccurrenceID, StoreID: i.StoreID}
}
func toRange(s ir.SourceSpan) sourceRange {
	return sourceRange{Start: position{Line: max(0, s.Start.Line-1), Character: max(0, s.Start.Column-1)}, End: position{Line: max(0, s.End.Line-1), Character: max(0, s.End.Column-1)}}
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func occurrenceKey(o occurrenceRecord) string {
	return stringKey(o.Range) + "\x00" + o.Symbol + "\x00" + o.Identity.OccurrenceID + "\x00" + strings.Join(o.Roles, ",")
}
func stringKey(r sourceRange) string {
	return strconv.Itoa(r.Start.Line) + ":" + strconv.Itoa(r.Start.Character) + ":" + strconv.Itoa(r.End.Line) + ":" + strconv.Itoa(r.End.Character)
}
func diagnosticKey(d diagnosticRecord) string {
	return stringKey(d.Range) + "\x00" + d.Code + "\x00" + d.Severity + "\x00" + d.Message
}
