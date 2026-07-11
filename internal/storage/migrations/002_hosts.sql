CREATE TABLE hosts (id TEXT PRIMARY KEY, identity_hash TEXT NOT NULL UNIQUE, name TEXT, updated_at TEXT NOT NULL);
CREATE TABLE boot_sessions (id INTEGER PRIMARY KEY, host_id TEXT NOT NULL REFERENCES hosts(id), boot_identity TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT, UNIQUE(host_id, boot_identity));
