package model

import "time"

type BranchSession struct {
	ID               string    `json:"id"`
	BaseRevision     string    `json:"base_revision"`
	LatestGeneration int       `json:"latest_generation"`
	ToolchainProfile string    `json:"toolchain_profile"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type RevisionGeneration struct {
	BranchID         string    `json:"branch_id"`
	Generation       int       `json:"generation"`
	ParentGeneration *int      `json:"parent_generation,omitempty"`
	SourceTreeDigest string    `json:"source_tree_digest"`
	CreatedAt        time.Time `json:"created_at"`
	Sealed           bool      `json:"sealed"`
	CommitSHA        string    `json:"commit_sha,omitempty"`
}

type QueryDefinition struct {
	Kind     string `json:"kind"`
	Subject  string `json:"subject"`
	Category string `json:"category"`
}

func (q QueryDefinition) ID() string { return q.Kind + "(" + q.Subject + ")" }

type QueryValue struct {
	Definition             QueryDefinition `json:"query_definition"`
	ComputationFingerprint string          `json:"computation_fingerprint"`
	OutputDigest           string          `json:"output_digest"`
	SemanticDigest         string          `json:"semantic_digest"`
	Generation             int             `json:"generation"`
	Dependencies           []string        `json:"dependencies"`
	CreatedAt              time.Time       `json:"created_at"`
}

type Decision string

const (
	DecisionReused      Decision = "REUSED"
	DecisionRecomputed  Decision = "RECOMPUTED"
	DecisionRetained    Decision = "RETAINED"
	DecisionInvalidated Decision = "INVALIDATED"
	DecisionRequired    Decision = "REQUIRED"
)

type Explanation struct {
	QueryID        string   `json:"query_id"`
	Decision       Decision `json:"decision"`
	Reason         string   `json:"reason"`
	Fingerprint    string   `json:"fingerprint,omitempty"`
	OutputDigest   string   `json:"output_digest,omitempty"`
	SemanticDigest string   `json:"semantic_digest,omitempty"`
}

type GenerationReport struct {
	BranchID     string        `json:"branch_id"`
	Generation   int           `json:"generation"`
	Profile      string        `json:"profile"`
	DurationMS   int64         `json:"duration_ms"`
	Explanations []Explanation `json:"explanations"`
}
