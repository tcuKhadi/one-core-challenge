# API Documentation

The app for the challenge: a tiny social app with posts, likes, comments, and profiles.

## Basics
- Base URL: `http://<host>:8080`
- All request and response bodies are JSON (`Content-Type: application/json`).
- `id` of a post is an integer. `username` is a lowercase string, letters, numbers, and underscores only, 1 to 32 characters.
- Post content is 1 to 280 characters. Comment content is 1 to 280 characters.
- Unknown path returns `404`. Known path with wrong method returns `405`.
- Error body is always `{"error": "message"}`.
- Storage is SQLite, inside the app.

## Endpoints

### 1. GET /feed
Newest posts first. Replies `200`:

```json
{
  "posts": [
    { "id": 7, "username": "mia", "content": "hello world", "likes": 3, "comments": 1, "created_at": "2026-11-06T10:00:00Z" },
    { "id": 6, "username": "sam", "content": "second post", "likes": 0, "comments": 0, "created_at": "2026-11-06T09:30:00Z" }
  ]
}
```

- Sorted by `created_at`, newest first.
- No pagination for the challenge build. An empty feed returns `{"posts": []}`.

### 2. POST /posts
Create a post. Request body:

```json
{ "username": "mia", "content": "hello world" }
```

Replies `201` with the created post:

```json
{ "id": 7, "username": "mia", "content": "hello world", "likes": 0, "comments": 0, "created_at": "2026-11-06T10:00:00Z" }
```

- Missing or invalid `username` or `content` returns `400` with `{"error": "..."}`.
- The post is saved to disk before the reply is sent.

### 3. GET /posts/:id
One post, with its like and comment counts. Replies `200`:

```json
{ "id": 7, "username": "mia", "content": "hello world", "likes": 3, "comments": 1, "created_at": "2026-11-06T10:00:00Z" }
```

- Unknown `id` returns `404` with `{"error": "post not found"}`.

### 4. POST /posts/:id/like
Like a post. No request body needed.

- Replies `201` with the new total: `{"post_id": 7, "likes": 4}`
- One request adds exactly one like. Liking twice is allowed and counts twice.
- Unknown `id` returns `404` with `{"error": "post not found"}`.
- The like is saved to disk before the reply is sent (durable write).

### 5. POST /posts/:id/comments
Add a comment. Request body:

```json
{ "username": "sam", "content": "nice post" }
```

Replies `201` with the created comment:

```json
{ "post_id": 7, "username": "sam", "content": "nice post", "created_at": "2026-11-06T10:05:00Z" }
```

- Missing or invalid `username` or `content` returns `400` with `{"error": "..."}`.
- Unknown `id` returns `404` with `{"error": "post not found"}`.
- The comment is saved to disk before the reply is sent.

### 6. GET /posts/:id/comments
List a post's comments, oldest first. Replies `200`:

```json
{
  "comments": [
    { "post_id": 7, "username": "sam", "content": "nice post", "created_at": "2026-11-06T10:05:00Z" }
  ]
}
```

- No comments yet returns `{"comments": []}`.
- Unknown `id` returns `404` with `{"error": "post not found"}`.

### 7. GET /users/:username
Profile page. Each successful request counts as one profile visit. Replies `200`:

```json
{ "username": "mia", "post_count": 4, "profile_visits": 37 }
```

- The visit count is incremented and saved to disk before the reply is sent.
- Unknown `username` returns `404` with `{"error": "user not found"}`.

## Status code summary

| Code | When |
| --- | --- |
| 200 | Successful read |
| 201 | Successful create or like |
| 400 | Invalid request body |
| 404 | Unknown path, post, or user |
| 405 | Known path, wrong method |
