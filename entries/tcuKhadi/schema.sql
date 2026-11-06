-- Reference solution schema (single source of truth, embedded into the binary).
-- Entries may design their own schema; api.md does not mandate tables.
-- Storage rules: in-process SQLite, journal_mode=WAL, synchronous=FULL.

CREATE TABLE IF NOT EXISTS posts (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	username   TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS likes (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id    INTEGER NOT NULL,
	username   TEXT NOT NULL,
	content    TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	username       TEXT PRIMARY KEY,
	profile_visits INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_likes_post    ON likes(post_id);
CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id);
CREATE INDEX IF NOT EXISTS idx_posts_user    ON posts(username);
