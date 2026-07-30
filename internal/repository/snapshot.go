package repository

import "context"

// UpsertWorkspaceSnapshot commits every document and dependent row in one transaction.
// A cancellation or failed row leaves the previous committed workspace untouched.
func (r *Repository) UpsertWorkspaceSnapshot(ctx context.Context, docs []DocumentSnapshot) error {
	if r == nil || r.db == nil {
		return ErrNilDatabase
	}
	if err := ctxErr(ctx); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, doc := range docs {
		if doc.DocumentPath() == "" {
			return ErrEmptyPath
		}
		doc = normalizeSnapshot(doc)
		if err := upsertDocument(ctx, tx, doc); err != nil {
			return err
		}
		if err := upsertSymbols(ctx, tx, doc.DocumentPath(), doc.Symbols); err != nil {
			return err
		}
		if err := upsertReferences(ctx, tx, doc.DocumentPath(), doc.References); err != nil {
			return err
		}
		if err := upsertDiagnostics(ctx, tx, doc.DocumentPath(), doc.Diagnostics); err != nil {
			return err
		}
		if err := upsertDependencyEdges(ctx, tx, doc.DocumentPath(), doc.DependencyIn); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
