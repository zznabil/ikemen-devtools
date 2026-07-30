package repository

import "fmt"

const (
	// CurrentSchemaVersion is the highest supported repository schema version.
	CurrentSchemaVersion = 2

	schemaVersionTable  = "ikm_schema_version"
	documentTable       = "ikm_documents"
	symbolTable         = "ikm_symbols"
	referenceTable      = "ikm_references"
	diagnosticTable     = "ikm_diagnostics"
	dependencyEdgeTable = "ikm_dependency_edges"
)

// Migration describes one schema upgrade step.
//
// The SQL statements are provided as-is and should be executed in order.
type Migration struct {
	Version    int
	Name       string
	Statements []string
}

// Migrations holds the repository schema migration plan.
var Migrations = []Migration{
	{
		Version: 1,
		Name:    "bootstrap",
		Statements: []string{
			"CREATE TABLE IF NOT EXISTS " + schemaVersionTable + " (\n" +
				"    version INTEGER NOT NULL PRIMARY KEY\n" +
				");",
			"INSERT INTO " + schemaVersionTable + " (version) VALUES (1);",
			"CREATE TABLE IF NOT EXISTS " + documentTable + " (\n" +
				"    path TEXT PRIMARY KEY NOT NULL,\n" +
				"    file_type TEXT NOT NULL,\n" +
				"    version TEXT NOT NULL,\n" +
				"    content_hash TEXT NOT NULL,\n" +
				"    updated_at INTEGER NOT NULL\n" +
				");",
			"CREATE TABLE IF NOT EXISTS " + symbolTable + " (\n" +
				"    id TEXT NOT NULL,\n" +
				"    document_path TEXT NOT NULL,\n" +
				"    kind TEXT NOT NULL,\n" +
				"    name TEXT NOT NULL,\n" +
				"    section TEXT NOT NULL,\n" +
				"    raw TEXT NOT NULL,\n" +
				"    start_line INTEGER NOT NULL,\n" +
				"    start_column INTEGER NOT NULL,\n" +
				"    end_line INTEGER NOT NULL,\n" +
				"    end_column INTEGER NOT NULL,\n" +
				"    PRIMARY KEY (document_path, id, start_line, start_column, end_line, end_column),\n" +
				"    FOREIGN KEY(document_path) REFERENCES " + documentTable + "(path) ON DELETE CASCADE\n" +
				");",
			"CREATE TABLE IF NOT EXISTS " + referenceTable + " (\n" +
				"    id TEXT NOT NULL,\n" +
				"    document_path TEXT NOT NULL,\n" +
				"    kind TEXT NOT NULL,\n" +
				"    name TEXT NOT NULL,\n" +
				"    raw TEXT NOT NULL,\n" +
				"    source_symbol TEXT NOT NULL,\n" +
				"    target TEXT NOT NULL,\n" +
				"    target_symbol_id TEXT,\n" +
				"    target_path TEXT,\n" +
				"    classification TEXT NOT NULL,\n" +
				"    resolved INTEGER NOT NULL,\n" +
				"    is_dynamic INTEGER NOT NULL,\n" +
				"    start_line INTEGER NOT NULL,\n" +
				"    start_column INTEGER NOT NULL,\n" +
				"    end_line INTEGER NOT NULL,\n" +
				"    end_column INTEGER NOT NULL,\n" +
				"    PRIMARY KEY (document_path, id, source_symbol, start_line, start_column, end_line, end_column),\n" +
				"    FOREIGN KEY(document_path) REFERENCES " + documentTable + "(path) ON DELETE CASCADE\n" +
				");",
			"CREATE TABLE IF NOT EXISTS " + diagnosticTable + " (\n" +
				"    path TEXT NOT NULL,\n" +
				"    code TEXT NOT NULL,\n" +
				"    severity TEXT NOT NULL,\n" +
				"    message TEXT NOT NULL,\n" +
				"    related_symbol TEXT,\n" +
				"    start_line INTEGER NOT NULL,\n" +
				"    start_column INTEGER NOT NULL,\n" +
				"    end_line INTEGER NOT NULL,\n" +
				"    end_column INTEGER NOT NULL,\n" +
				"    PRIMARY KEY (path, code, severity, message, start_line, start_column, end_line, end_column),\n" +
				"    FOREIGN KEY(path) REFERENCES " + documentTable + "(path) ON DELETE CASCADE\n" +
				");",
			"CREATE INDEX IF NOT EXISTS " + symbolTable + "_document_path_idx ON " + symbolTable + " (document_path);",
			"CREATE INDEX IF NOT EXISTS " + referenceTable + "_document_path_idx ON " + referenceTable + " (document_path);",
			"CREATE INDEX IF NOT EXISTS " + diagnosticTable + "_path_idx ON " + diagnosticTable + " (path);",
		},
	},
	{
		Version: 2,
		Name:    "dependency-edges",
		Statements: []string{
			"ALTER TABLE " + documentTable + " ADD COLUMN generation INTEGER NOT NULL DEFAULT 0;",
			"CREATE TABLE IF NOT EXISTS " + dependencyEdgeTable + " (\n" +
				"    source_path TEXT NOT NULL,\n" +
				"    target_path TEXT NOT NULL,\n" +
				"    edge_type TEXT NOT NULL,\n" +
				"    PRIMARY KEY (source_path, target_path, edge_type),\n" +
				"    FOREIGN KEY(source_path) REFERENCES " + documentTable + "(path) ON DELETE CASCADE\n" +
				");",
			"CREATE INDEX IF NOT EXISTS " + dependencyEdgeTable + "_source_idx ON " + dependencyEdgeTable + " (source_path);",
			"CREATE INDEX IF NOT EXISTS " + dependencyEdgeTable + "_target_idx ON " + dependencyEdgeTable + " (target_path);",
			"DELETE FROM " + schemaVersionTable + ";",
			"INSERT INTO " + schemaVersionTable + " (version) VALUES (2);",
		},
	},
}

// MigrationSQL returns a copy of the migration SQL statements.
func MigrationSQL() []Migration {
	out := make([]Migration, len(Migrations))
	for i := range Migrations {
		out[i] = Migrations[i]
		out[i].Statements = append([]string{}, Migrations[i].Statements...)
	}
	return out
}

func migrationByVersion(version int) (Migration, error) {
	for _, migration := range Migrations {
		if migration.Version == version {
			copy := Migration{
				Version:    migration.Version,
				Name:       migration.Name,
				Statements: append([]string{}, migration.Statements...),
			}
			return copy, nil
		}
	}
	return Migration{}, fmt.Errorf("repository: unknown migration version %d", version)
}
