CREATE TABLE user_preferences (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK(schema_version = 1),
    theme TEXT NOT NULL CHECK(theme IN ('system','dark','light')),
    density TEXT NOT NULL CHECK(density IN ('comfortable','compact')),
    pinned_resources_json TEXT NOT NULL DEFAULT '[]' CHECK(length(pinned_resources_json) <= 4096),
    landing_page TEXT NOT NULL CHECK(landing_page IN ('watch','resources','server','events','alerts')),
    chart_range TEXT NOT NULL CHECK(chart_range IN ('1h','6h','24h','7d','30d')),
    updated_at INTEGER NOT NULL
);
