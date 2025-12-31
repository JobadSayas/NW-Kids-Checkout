CREATE TABLE schema_migrations (version uint64,dirty bool);
CREATE UNIQUE INDEX version_unique ON schema_migrations (version);
CREATE TABLE locations
(
    id                 INTEGER PRIMARY KEY,
    planning_center_id TEXT NOT NULL UNIQUE,
    name               TEXT NOT NULL
);
CREATE INDEX idx_name ON locations (name);
CREATE TABLE checkins
(
    id                 INTEGER PRIMARY KEY,
    planning_center_id TEXT    NOT NULL UNIQUE,
    location_id        INTEGER NOT NULL,
    first_name         TEXT    NOT NULL,
    last_name          TEXT    NOT NULL,
    security_code      TEXT    NOT NULL,
    checked_out_at     DATETIME DEFAULT NULL
);
CREATE INDEX idx_checked_out_at ON checkins (checked_out_at);
