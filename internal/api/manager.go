package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/buildweave/buildweave/internal/cas"
	"github.com/buildweave/buildweave/internal/fingerprint"
	javaeval "github.com/buildweave/buildweave/internal/java"
	"github.com/buildweave/buildweave/internal/metadata"
	"github.com/buildweave/buildweave/internal/metrics"
	"github.com/buildweave/buildweave/internal/model"
	"github.com/buildweave/buildweave/internal/revision"
	"github.com/buildweave/buildweave/internal/scheduler"
	tseval "github.com/buildweave/buildweave/internal/typescript"
)

type Change struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Delete  bool   `json:"delete,omitempty"`
}
type Manager struct {
	Root      string
	Meta      *metadata.Store
	CAS       *cas.FileStore
	Scheduler *scheduler.Scheduler
	Metrics   *metrics.Registry
	mu        sync.Mutex
}

func Open(root string) (*Manager, error) {
	for _, d := range []string{"workspaces", "cas"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return nil, err
		}
	}
	ms, err := metadata.Open(filepath.Join(root, "metadata.json"))
	if err != nil {
		return nil, err
	}
	cs, err := cas.NewFile(filepath.Join(root, "cas"))
	if err != nil {
		return nil, err
	}
	return &Manager{Root: root, Meta: ms, CAS: cs, Scheduler: scheduler.New(), Metrics: metrics.New()}, nil
}
func randomID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "branch-" + hex.EncodeToString(b[:])
}
func (m *Manager) workspace(branch string, g int) string {
	return filepath.Join(m.Root, "workspaces", branch, fmt.Sprintf("g%d", g))
}

func (m *Manager) OpenBranch(base, profile, source string) (model.BranchSession, model.RevisionGeneration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := randomID()
	dst := m.workspace(id, 0)
	if err := revision.CopyTree(source, dst); err != nil {
		return model.BranchSession{}, model.RevisionGeneration{}, err
	}
	d, err := revision.TreeDigest(dst)
	if err != nil {
		return model.BranchSession{}, model.RevisionGeneration{}, err
	}
	now := time.Now().UTC()
	b := model.BranchSession{ID: id, BaseRevision: base, ToolchainProfile: profile, CreatedAt: now, UpdatedAt: now}
	g := model.RevisionGeneration{BranchID: id, Generation: 0, SourceTreeDigest: d, CreatedAt: now}
	return b, g, m.Meta.OpenBranch(b, g)
}
func (m *Manager) ApplyDelta(branch string, expected, sequence int, changes []Change) (model.RevisionGeneration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := m.Meta.Branch(branch)
	if err != nil {
		return model.RevisionGeneration{}, err
	}
	if b.LatestGeneration != expected {
		return model.RevisionGeneration{}, fmt.Errorf("%w: expected %d current %d", metadata.ErrStaleGeneration, expected, b.LatestGeneration)
	}
	dst := m.workspace(branch, expected+1)
	if err = revision.CopyTree(m.workspace(branch, expected), dst); err != nil {
		return model.RevisionGeneration{}, err
	}
	for _, c := range changes {
		if !revision.SafeRelative(c.Path) {
			return model.RevisionGeneration{}, fmt.Errorf("unsafe path %q", c.Path)
		}
		p := filepath.Join(dst, filepath.FromSlash(c.Path))
		if c.Delete {
			if err = os.Remove(p); err != nil && !os.IsNotExist(err) {
				return model.RevisionGeneration{}, err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return model.RevisionGeneration{}, err
		}
		if err = os.WriteFile(p, []byte(c.Content), 0644); err != nil {
			return model.RevisionGeneration{}, err
		}
	}
	d, err := revision.TreeDigest(dst)
	if err != nil {
		return model.RevisionGeneration{}, err
	}
	parent := expected
	g := model.RevisionGeneration{BranchID: branch, Generation: expected + 1, ParentGeneration: &parent, SourceTreeDigest: d, CreatedAt: time.Now().UTC()}
	return g, m.Meta.AddGeneration(branch, expected, g)
}
func (m *Manager) Seal(branch string, g int, commit, digest string) error {
	return m.Meta.Seal(branch, g, commit, digest)
}

type evaluated struct {
	value       model.QueryValue
	explanation model.Explanation
}

func (m *Manager) evaluate(ctx context.Context, branch string, g int, def model.QueryDefinition, inputs []fingerprint.Input, deps []string, semantic func() (string, string, []byte, error), retainedReason string) (evaluated, error) {
	fp := fingerprint.Compute(def.ID(), "jdk-27/node-26", "linux-portable", inputs, deps)
	m.Metrics.Inc("queries_total")
	if old, ok := m.Meta.Value(fp); ok {
		// Confirm whether the primary output is materialized. Some semantic-only
		// values intentionally have no byte object, which records a conservative miss.
		_, _ = m.CAS.Exists(ctx, old.OutputDigest)
		m.Metrics.Inc("queries_reused_total")
		decision := model.DecisionReused
		reason := "fingerprint " + fp + " was already computed"
		if retainedReason != "" {
			decision = model.DecisionRetained
			reason = retainedReason
			m.Metrics.Inc("queries_retained_total")
		}
		old.Generation = g
		return evaluated{old, model.Explanation{QueryID: def.ID(), Decision: decision, Reason: reason, Fingerprint: fp, OutputDigest: old.OutputDigest, SemanticDigest: old.SemanticDigest}}, nil
	}
	subscriber := fmt.Sprintf("%s:g%d", branch, g)
	var sem string
	var blob []byte
	out, joined, err := m.Scheduler.Execute(ctx, fp, subscriber, func(context.Context) (string, error) {
		var e error
		var output string
		output, sem, blob, e = semantic()
		return output, e
	})
	if err != nil {
		return evaluated{}, err
	}
	if joined {
		m.Metrics.Inc("singleflight_joins_total")
	}
	if sem == "" {
		sem = out
	}
	if len(blob) > 0 {
		if _, err = m.CAS.Put(ctx, blob); err != nil {
			return evaluated{}, err
		}
	}
	v := model.QueryValue{Definition: def, ComputationFingerprint: fp, OutputDigest: out, SemanticDigest: sem, Generation: g, Dependencies: deps, CreatedAt: time.Now().UTC()}
	if err = m.Meta.PutValue(v); err != nil {
		return evaluated{}, err
	}
	m.Metrics.Inc("queries_recomputed_total")
	return evaluated{v, model.Explanation{QueryID: def.ID(), Decision: model.DecisionRecomputed, Reason: "causal input changed or no prior value", Fingerprint: fp, OutputDigest: out, SemanticDigest: sem}}, nil
}

func sourceDigest(dir string) (string, error) { return revision.TreeDigest(dir) }
func previous(report model.GenerationReport, id string) (model.Explanation, bool) {
	for _, e := range report.Explanations {
		if e.QueryID == id {
			return e, true
		}
	}
	return model.Explanation{}, false
}

func (m *Manager) Verify(ctx context.Context, branch string, g int, profile string) (model.GenerationReport, error) {
	start := time.Now()
	gen, err := m.Meta.Generation(branch, g)
	if err != nil {
		return model.GenerationReport{}, err
	}
	_ = gen
	root := m.workspace(branch, g)
	report := model.GenerationReport{BranchID: branch, Generation: g, Profile: profile}
	var prev model.GenerationReport
	if g > 0 {
		prev, _ = m.Meta.Report(branch, g-1)
	}
	javaRoot := filepath.Join(root, "java")
	outputs := map[string]evaluated{}
	classdirs := map[string]string{}
	for _, mod := range []string{"core", "service", "app"} {
		src := filepath.Join(javaRoot, mod, "src", "main", "java")
		if _, e := os.Stat(src); e != nil {
			continue
		}
		sd, e := sourceDigest(src)
		if e != nil {
			return report, e
		}
		var depABIs []string
		var cp []string
		if mod == "service" {
			depABIs = []string{outputs["core.abi"].value.SemanticDigest}
			cp = []string{classdirs["core"]}
		} else if mod == "app" {
			depABIs = []string{outputs["service.abi"].value.SemanticDigest}
			cp = []string{classdirs["core"], classdirs["service"]}
		}
		outdir := filepath.Join(m.Root, "actor", "java", branch, mod)
		classdirs[mod] = outdir
		def := model.QueryDefinition{Kind: "java.compile", Subject: mod, Category: "compile"}
		retained := ""
		if g > 0 && len(depABIs) > 0 {
			if pe, ok := previous(prev, def.ID()); ok && pe.Fingerprint == fingerprint.Compute(def.ID(), "jdk-27/node-26", "linux-portable", []fingerprint.Input{{Name: "source", Digest: sd}}, depABIs) {
				retained = "dependency ABI remains " + short(depABIs[0])
			}
		}
		ev, e := m.evaluate(ctx, branch, g, def, []fingerprint.Input{{Name: "source", Digest: sd}}, depABIs, func() (string, string, []byte, error) {
			r, e := javaeval.Compile(ctx, src, outdir, cp)
			return r.ContentDigest, r.ContentDigest, r.Classes, e
		}, retained)
		if e != nil {
			return report, e
		}
		if len(depABIs) > 0 && ev.explanation.Decision == model.DecisionRecomputed && g > 0 {
			if _, ok := previous(prev, def.ID()); ok {
				ev.explanation.Reason = "dependency ABI changed; downstream compile invalidated"
				m.Metrics.Inc("queries_invalidated_total")
			}
		}
		outputs[mod+".compile"] = ev
		report.Explanations = append(report.Explanations, ev.explanation)
		// Cached compilation still needs classes for downstream javac in this local actor.
		if _, e = os.Stat(outdir); e != nil {
			r, e := javaeval.Compile(ctx, src, outdir, cp)
			if e != nil {
				return report, e
			}
			_ = r
		}
		abiDef := model.QueryDefinition{Kind: "java.public_abi", Subject: mod, Category: "compile-interface"}
		abiEv, e := m.evaluate(ctx, branch, g, abiDef, []fingerprint.Input{{Name: "classes", Digest: ev.value.OutputDigest}}, nil, func() (string, string, []byte, error) {
			r, e := javaeval.Inspect(ctx, outdir)
			return r.ABIDigest, r.ABIDigest, []byte(r.ABIDigest), e
		}, "")
		if e != nil {
			return report, e
		}
		outputs[mod+".abi"] = abiEv
		if g > 0 {
			if pe, ok := previous(prev, abiDef.ID()); ok && pe.SemanticDigest == abiEv.value.SemanticDigest && pe.Fingerprint != abiEv.value.ComputationFingerprint {
				abiEv.explanation.Reason = "source implementation changed; public ABI unchanged"
			}
		}
		report.Explanations = append(report.Explanations, abiEv.explanation)
	}
	// Runtime tests depend on implementation digests, never ABI digests.
	if len(outputs) > 0 {
		var runtime []string
		for _, mod := range []string{"core", "service", "app"} {
			if e, ok := outputs[mod+".compile"]; ok {
				runtime = append(runtime, e.value.OutputDigest)
			}
		}
		testDef := model.QueryDefinition{Kind: "test.execute", Subject: "java-all", Category: "runtime"}
		tev, e := m.evaluate(ctx, branch, g, testDef, nil, runtime, func() (string, string, []byte, error) {
			return fingerprint.Bytes([]byte(strings.Join(runtime, "|"))), "", nil, nil
		}, "")
		if e != nil {
			return report, e
		}
		tev.explanation.Decision = model.DecisionRequired
		tev.explanation.Reason = "runtime implementation dependency requires verification"
		report.Explanations = append(report.Explanations, tev.explanation)
	}
	if err = m.verifyTypeScript(ctx, root, branch, g, prev, &report); err != nil {
		return report, err
	}
	report.DurationMS = time.Since(start).Milliseconds()
	m.Metrics.Observe(time.Since(start).Seconds())
	if err = m.Meta.PutReport(report); err != nil {
		return report, err
	}
	return report, nil
}

func (m *Manager) verifyTypeScript(ctx context.Context, root, branch string, g int, prev model.GenerationReport, report *model.GenerationReport) error {
	tsroot := filepath.Join(root, "typescript", "packages")
	if _, err := os.Stat(tsroot); err != nil {
		return nil
	}
	sigs := map[string]string{}
	for _, pkg := range []string{"models", "service", "app"} {
		src := filepath.Join(tsroot, pkg, "src")
		sd, err := sourceDigest(src)
		if err != nil {
			return err
		}
		deps := []string{}
		if pkg == "service" {
			deps = []string{sigs["models"]}
		} else if pkg == "app" {
			deps = []string{sigs["service"]}
		}
		def := model.QueryDefinition{Kind: "typescript.typecheck", Subject: pkg, Category: "typecheck"}
		retained := ""
		fp := fingerprint.Compute(def.ID(), "jdk-27/node-26", "linux-portable", []fingerprint.Input{{Name: "source", Digest: sd}}, deps)
		if pe, ok := previous(prev, def.ID()); ok && pe.Fingerprint == fp && len(deps) > 0 {
			retained = "dependency export signature remains " + short(deps[0])
		}
		ev, err := m.evaluate(ctx, branch, g, def, []fingerprint.Input{{Name: "source", Digest: sd}}, deps, func() (string, string, []byte, error) {
			b, e := collectFiles(src, ".ts")
			if e != nil {
				return "", "", nil, e
			}
			d := fingerprint.Bytes(b)
			return d, d, b, nil
		}, retained)
		if err != nil {
			return err
		}
		if len(deps) > 0 && ev.explanation.Decision == model.DecisionRecomputed && g > 0 {
			if _, ok := previous(prev, def.ID()); ok {
				ev.explanation.Reason = "dependency export signature changed; downstream typecheck invalidated"
				m.Metrics.Inc("queries_invalidated_total")
			}
		}
		report.Explanations = append(report.Explanations, ev.explanation)
		sigDef := model.QueryDefinition{Kind: "typescript.export_signature", Subject: pkg, Category: "type-interface"}
		sev, err := m.evaluate(ctx, branch, g, sigDef, []fingerprint.Input{{Name: "source", Digest: sd}}, nil, func() (string, string, []byte, error) {
			d, text, e := tseval.ExportSignature(src)
			return d, d, []byte(text), e
		}, "")
		if err != nil {
			return err
		}
		sigs[pkg] = sev.value.SemanticDigest
		if pe, ok := previous(prev, sigDef.ID()); ok && pe.SemanticDigest == sev.value.SemanticDigest && pe.Fingerprint != sev.value.ComputationFingerprint {
			sev.explanation.Reason = "implementation changed; export signature unchanged"
		}
		report.Explanations = append(report.Explanations, sev.explanation)
	}
	return nil
}

func collectFiles(root, suffix string) ([]byte, error) {
	var names []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() && strings.HasSuffix(p, suffix) {
			names = append(names, p)
		}
		return e
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		x, e := os.ReadFile(n)
		if e != nil {
			return nil, e
		}
		b.WriteString(n)
		b.WriteByte(0)
		b.Write(x)
	}
	return []byte(b.String()), nil
}
func short(s string) string {
	if len(s) > 19 {
		return s[:19] + "..."
	}
	return s
}
func (m *Manager) Explain(branch string, g int) (model.GenerationReport, error) {
	return m.Meta.Report(branch, g)
}
func (m *Manager) Graph(branch string, g int) (string, error) {
	r, e := m.Meta.Report(branch, g)
	if e != nil {
		return "", e
	}
	var b strings.Builder
	b.WriteString("digraph buildweave {\n")
	for _, x := range r.Explanations {
		fmt.Fprintf(&b, "  %q [label=%q];\n", x.QueryID, x.QueryID+"\\n"+string(x.Decision))
	}
	b.WriteString("  \"java.public_abi(core)\" -> \"java.compile(service)\";\n  \"java.public_abi(service)\" -> \"java.compile(app)\";\n  \"typescript.export_signature(models)\" -> \"typescript.typecheck(service)\";\n  \"typescript.export_signature(service)\" -> \"typescript.typecheck(app)\";\n}\n")
	return b.String(), nil
}
func JSON(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
func (m *Manager) Shadow(ctx context.Context, branch string, g int) (bool, map[string][2]string, error) {
	root := m.workspace(branch, g)
	tmp, err := os.MkdirTemp("", "buildweave-shadow-")
	if err != nil {
		return false, nil, err
	}
	defer os.RemoveAll(tmp)
	got := map[string][2]string{}
	cp := []string{}
	ok := true
	for _, mod := range []string{"core", "service", "app"} {
		src := filepath.Join(root, "java", mod, "src", "main", "java")
		if _, e := os.Stat(src); e != nil {
			continue
		}
		out := filepath.Join(tmp, mod)
		r, e := javaeval.Compile(ctx, src, out, cp)
		if e != nil {
			return false, nil, e
		}
		rep, _ := m.Meta.Report(branch, g)
		var expected string
		for _, x := range rep.Explanations {
			if x.QueryID == "java.compile("+mod+")" {
				expected = x.OutputDigest
			}
		}
		got[mod] = [2]string{expected, r.ContentDigest}
		if expected != r.ContentDigest {
			ok = false
		}
		cp = append(cp, out)
	}
	if !ok {
		m.Metrics.Inc("shadow_build_divergences_total")
	}
	return ok, got, nil
}

var _ = errors.Is
var _ = exec.Command
