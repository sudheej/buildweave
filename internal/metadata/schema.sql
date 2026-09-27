CREATE TABLE IF NOT EXISTS branch_sessions (
  id TEXT PRIMARY KEY, base_revision TEXT NOT NULL, latest_generation BIGINT NOT NULL,
  toolchain_profile TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS revision_generations (
  branch_id TEXT NOT NULL REFERENCES branch_sessions(id), generation BIGINT NOT NULL,
  parent_generation BIGINT, source_tree_digest TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL,
  sealed BOOLEAN NOT NULL DEFAULT FALSE, commit_sha TEXT, PRIMARY KEY(branch_id,generation)
);
CREATE TABLE IF NOT EXISTS query_definitions (id TEXT PRIMARY KEY, kind TEXT NOT NULL, subject TEXT NOT NULL, category TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS query_values (
  fingerprint TEXT PRIMARY KEY, query_id TEXT NOT NULL, output_digest TEXT NOT NULL,
  semantic_digest TEXT NOT NULL, generation BIGINT NOT NULL, dependencies JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS subscriptions (fingerprint TEXT NOT NULL, branch_id TEXT NOT NULL, generation BIGINT NOT NULL, PRIMARY KEY(fingerprint,branch_id,generation));
CREATE TABLE IF NOT EXISTS execution_attempts (id BIGSERIAL PRIMARY KEY, fingerprint TEXT NOT NULL, output_digest TEXT, state TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS sealed_revisions (branch_id TEXT NOT NULL, generation BIGINT NOT NULL, commit_sha TEXT NOT NULL, source_tree_digest TEXT NOT NULL, PRIMARY KEY(branch_id,generation));

