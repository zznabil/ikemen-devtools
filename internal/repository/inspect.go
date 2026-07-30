package repository

import (
	"context"
	"encoding/json"
)

type Inspection struct {
	Document        DocumentSnapshot `json:"document"`
	SymbolCount     int              `json:"symbolCount"`
	ReferenceCount  int              `json:"referenceCount"`
	DiagnosticCount int              `json:"diagnosticCount"`
}

func (r *Repository) Inspect(ctx context.Context, path string) (Inspection, error) {
	d, err := r.ReadDocumentSnapshot(ctx, path)
	if err != nil {
		return Inspection{}, err
	}
	return Inspection{Document: d, SymbolCount: len(d.Symbols), ReferenceCount: len(d.References), DiagnosticCount: len(d.Diagnostics)}, nil
}
func (r *Repository) ExportSnapshot(ctx context.Context, path string) ([]byte, error) {
	d, err := r.ReadDocumentSnapshot(ctx, path)
	if err != nil {
		return nil, err
	}
	return json.Marshal(d)
}
