package repository

// BuildEvidence records the acceptance gate for the embedded driver. Release CI may
// replace measured values; conservative ceilings keep regressions visible.
type BuildEvidence struct {
	CGORequired     bool   `json:"cgoRequired"`
	MaxBinaryBytes  int64  `json:"maxBinaryBytes"`
	MaxBuildSeconds int64  `json:"maxBuildSeconds"`
	Driver          string `json:"driver"`
}

var EmbeddedSQLiteEvidence = BuildEvidence{CGORequired: false, MaxBinaryBytes: 100 << 20, MaxBuildSeconds: 180, Driver: "modernc.org/sqlite@v1.54.0"}
