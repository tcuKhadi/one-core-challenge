-- Reference solution schema
CREATE TABLE IF NOT EXISTS users (
    username       TEXT PRIMARY KEY,
    profile_visits INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS posts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username   TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS likes (
    post_id  INTEGER NOT NULL,
    username TEXT NOT NULL,
    PRIMARY KEY (post_id, username)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER NOT NULL,
    username   TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id, id);
CREATE INDEX IF NOT EXISTS idx_posts_user    ON posts(username);
