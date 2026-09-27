package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type Input struct{ Name, Digest string }

// Compute intentionally has no revision or commit argument. Provenance is stored separately.
func Compute(definition, toolchain, environment string, inputs []Input, dependencies []string) string {
	in := append([]Input(nil), inputs...)
	deps := append([]string(nil), dependencies...)
	sort.Slice(in, func(i, j int) bool {
		if in[i].Name == in[j].Name {
			return in[i].Digest < in[j].Digest
		}
		return in[i].Name < in[j].Name
	})
	sort.Strings(deps)
	b, _ := json.Marshal(struct {
		Definition, Toolchain, Environment string
		Inputs                             []Input
		Dependencies                       []string
	}{definition, toolchain, environment, in, deps})
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

func Bytes(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
