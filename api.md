# API Documentation

The app for the challenge: a tiny social app with posts, likes, comments, and profiles.

## Basics
- Base URL: `http://<host>:8080`
- All request and response bodies are JSON (`Content-Type: application/json`).
- A post `id` is an integer. On a fresh database, ids are assigned 1, 2, 3, ... in creation order.
- `username` is a lowercase string: letters, numbers, and underscores only, 1 to 32 characters (`^[a-z0-9_]{1,32}$`).
- Post and comment `content` is 1 to 280 characters, counted as Unicode characters, not bytes. Content that is empty or only whitespace is invalid.
- Timestamps are UTC, RFC 3339, whole seconds, for example `2026-11-06T10:00:00Z`. Lists that sort by time break ties by creation order, newest first.
- Unknown path returns `404`. Known path with the wrong method returns `405`. Both use the standard error body.
- Error body is always `{"error": "message"}`. The checks only look at the status code and that `error` is a non-empty string; the message text is free.
- Storage is in-process SQLite. See [rules.md](rules.md) for how it must be used.
- A completed write is visible to the next read.

## Authentication
Three routes need a token: `POST /posts`, `POST /posts/:id/like`, and `POST /posts/:id/comments`. Every read is public.

- Header: `Authorization: Bearer <token>`
- The token is a JWT signed with HS256. The secret comes from the `JWT_SECRET` environment variable (benchmark default: `one-core-challenge-secret`).
- Payload: `sub` is the username acting, `exp` is the expiry in Unix epoch seconds.
- The server checks the signature and `exp` on every protected request. Any algorithm other than HS256, including `none`, is invalid.
- The acting username always comes from `sub`. Request bodies never carry a username.
- Missing header, or not a Bearer token: `401`.
- Bad signature, malformed token, expired token, or a `sub` that is not a valid username: `401`.
- There is no register or login endpoint. The test scripts create tokens ([lib/token.js](lib/token.js), [lib/token.sh](lib/token.sh)).

## Users
A user exists once they have made their first successful write: a post, a comment, or a like. Before that, `GET /users/:username` returns `404`.

## Order of checks
1. Route: `404` unknown path, `405` wrong method
2. Token: `401` (protected routes only)
3. Body: `400` (routes that take a body)
4. Post lookup: `404`

A `:id` that is not a positive integer returns `404`.

## Endpoints

### 1. GET /feed
The 20 newest posts, newest first. Sorted by `created_at` descending, then `id` descending. Replies `200`:

```json
{
  "posts": [
    { "id": 7, "username": "mia", "content": "hello world", "likes": 3, "comments": 1, "created_at": "2026-11-06T10:00:00Z" },
    { "id": 6, "username": "sam", "content": "second post", "likes": 0, "comments": 0, "created_at": "2026-11-06T09:30:00Z" }
  ]
}
```

- An empty feed returns `{"posts": []}`.

### 2. POST /posts
Protected (needs a token). Request body:

```json
{ "content": "hello world" }
```

Replies `201` with the created post:

```json
{
  "post": {
    "id": 7,
    "username": "mia",
    "content": "hello world",
    "likes": 0,
    "comments": 0,
    "created_at": "2026-11-06T10:00:00Z"
  }
}
```

- Missing or invalid `content` returns `400`.
- The post is committed and synced to disk before the reply is sent.

### 3. GET /posts/:id
One post, with its like and comment counts. Replies `200`:

```json
{
  "post": {
    "id": 7,
    "username": "mia",
    "content": "hello world",
    "likes": 3,
    "comments": 1,
    "created_at": "2026-11-06T10:00:00Z"
  }
}
```

- Unknown `id` returns `404`.

### 4. POST /posts/:id/like
Protected (needs a token). No request body. One user can like a post once.

- First like by this user on this post: `201`
- Liking again: `200`, and the count does not change

Both reply with the current total:

```json
{ "post_id": 7, "likes": 4 }
```

- Unknown `id` returns `404`.
- A first like is committed and synced to disk before the reply is sent.

### 5. POST /posts/:id/comments
Protected (needs a token). Request body:

```json
{ "content": "nice post" }
```

Replies `201` with the created comment:

```json
{
  "comment": {
    "post_id": 7,
    "username": "sam",
    "content": "nice post",
    "created_at": "2026-11-06T10:05:00Z"
  }
}
```

- Missing or invalid `content` returns `400`.
- Unknown `id` returns `404`.
- The comment is committed and synced to disk before the reply is sent.

### 6. GET /posts/:id/comments
The 20 newest comments on a post, newest first. Sorted by `created_at` descending, then comment creation order descending. Replies `200`:

```json
{
  "comments": [
    { "post_id": 7, "username": "sam", "content": "nice post", "created_at": "2026-11-06T10:05:00Z" }
  ]
}
```

- No comments yet returns `{"comments": []}`.
- Unknown `id` returns `404`.

### 7. GET /users/:username
Profile page. Each successful request counts as one profile visit. Replies `200`:

```json
{
  "user": {
    "username": "mia",
    "post_count": 4,
    "profile_visits": 37
  }
}
```

- `profile_visits` includes the visit being made right now.
- The visit is incremented and synced to disk before the reply is sent.
- Unknown `username` returns `404`.
- `post_count` is the total number of posts created by this user.

## Status code summary

| Code | When |
| --- | --- |
| 200 | Successful read, or repeat like |
| 201 | Successful create or first like |
| 400 | Invalid request body |
| 401 | Missing, malformed, or expired token |
| 404 | Unknown path, post, or user |
| 405 | Known path, wrong method |
