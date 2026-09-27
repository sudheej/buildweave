# Demo

Run `make demo`. The command makes a fresh isolated state directory and performs nine scenarios:

1. Cold Java and TypeScript query evaluation populates the CAS.
2. Identical verification obtains exact fingerprint hits.
3. A Java method body changes; core bytecode and its ABI query recompute, the ABI compares equal, and service/app compilation remains valid. Runtime verification is still demanded.
4. A public overload changes the ABI and forces downstream compilation fingerprints to change.
5. A TypeScript body-only edit preserves the export signature and downstream typechecks.
6. An exported optional parameter changes the signature and propagates.
7. Two generation subscribers join one deliberately blocked action. Generation 4 is removed while generation 5 still receives the result; the execution counter remains one.
8. The already-verified tree is sealed to `demo-commit-abc123`. Verification after sealing shows zero recomputations attributable to the SHA.
9. Fresh Java compilation in a temporary shadow directory is digest-compared with optimized values and must report `MATCH`.

All explanation lines are produced from stored fingerprints and values. The final summary reads live counters rather than fixed reuse counts.
