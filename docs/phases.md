# Implemented phases

## Phase 1

One persistent local repository actor retains generation workspaces, Java outputs, and CAS objects. Verification records timings and values. Repeat verification and clean shadow comparison demonstrate warm state without making mutable state authoritative.

## Phase 2

Compiled Java public ABI fingerprints, explicit compile/runtime dependency categories, reverse invalidation primitives, semantic early cutoff, subscriber sets, singleflight execution, and nondeterminism detection are implemented and tested.

## Phase 3

TypeScript export-signature values and downstream typecheck cutoff, content-addressed dependency-friendly workspace layout, the OpenBranch/ApplyDelta/Verify/Seal protocol, exact-tree pre-commit promotion, CLI explanations/graphs, metrics, and Tekton request examples are implemented.

## Not implemented: Phase 4+

There is no signing/evidence plane, checkpoint placement policy, distributed scheduler, remote execution, microVM snapshotting, or Kubernetes operator. Those are explicitly outside this prototype.
