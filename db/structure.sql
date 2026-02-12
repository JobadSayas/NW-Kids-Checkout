CREATE TABLE schema_migrations (version uint64,dirty bool);
CREATE UNIQUE INDEX version_unique ON schema_migrations (version);
CREATE TABLE location_groups
(
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);
CREATE TABLE locations
(
    id                        INTEGER PRIMARY KEY,
    planning_center_id        TEXT    NOT NULL UNIQUE,
    planning_center_parent_id TEXT     DEFAULT NULL,
    location_group_id         INTEGER  DEFAULT NULL,
    event_id                  INTEGER NOT NULL,
    name                      TEXT    NOT NULL,
    auto_fetch                INTEGER  DEFAULT 0,
    last_checked_out_time     DATETIME DEFAULT NULL
);
CREATE INDEX idx_name ON locations (name);
CREATE UNIQUE INDEX idx_planning_center_id ON locations (planning_center_id);
CREATE TABLE checkins
(
    id                 INTEGER PRIMARY KEY,
    planning_center_id TEXT    NOT NULL UNIQUE,
    location_id        INTEGER NOT NULL,
    first_name         TEXT    NOT NULL,
    last_name          TEXT    NOT NULL,
    security_code      TEXT    NOT NULL,
    checked_out_at     DATETIME DEFAULT NULL
, checked_out_confirmed_at DATETIME DEFAULT NULL);
CREATE INDEX idx_checked_out_at ON checkins (checked_out_at);
CREATE TABLE events
(
    id                 INTEGER PRIMARY KEY,
    name               TEXT NOT NULL UNIQUE,
    planning_center_id TEXT NOT NULL
);
CREATE TABLE fiber_storage (
			k  VARCHAR(64) PRIMARY KEY NOT NULL DEFAULT '',
			v  BLOB NOT NULL,
			e  BIGINT NOT NULL DEFAULT '0'
		);
CREATE INDEX e ON fiber_storage (e);
CREATE UNIQUE INDEX idx_location_groups_name ON location_groups (name);
