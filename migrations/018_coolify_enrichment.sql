CREATE TABLE coolify_enrichment_cache (
    cache_key TEXT PRIMARY KEY,
    payload_json TEXT NOT NULL CHECK(length(payload_json) <= 16777216),
    resource_count INTEGER NOT NULL CHECK(resource_count >= 0 AND resource_count <= 10000),
    fetched_at INTEGER NOT NULL
);
CREATE TABLE coolify_sync_state (
    id INTEGER PRIMARY KEY CHECK(id = 1),
    state TEXT NOT NULL CHECK(state IN ('unknown','healthy','degraded','down')),
    last_attempt_at INTEGER,
    last_success_at INTEGER,
    error_code TEXT
);
INSERT INTO coolify_sync_state(id,state) VALUES(1,'unknown');
CREATE TABLE coolify_deployments (
    deployment_uuid TEXT PRIMARY KEY,
    resource_uuid TEXT,
    last_status TEXT NOT NULL,
    commit_sha TEXT,
    commit_message TEXT,
    first_seen_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    event_emitted_at INTEGER
);
CREATE INDEX coolify_deployments_updated ON coolify_deployments(updated_at);
