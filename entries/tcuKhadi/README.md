# tcuKhadi reference solution

Go + Gin + SQLite (in-process). This is the reference entry, not a limit-pusher.

## Storage approach
- Single SQLite file `challenge.db` in the container working directory
- `journal_mode = WAL`, `synchronous = FULL`: every completed write is fsynced before the reply
- One pooled connection (`MaxOpenConns = 1`): commits are serialized, so behavior is deterministic. Reads queue behind writes at high load, which is fine for a reference.
- Every write is one transaction with one commit. A post, comment, or like also records the user in the same transaction, so there is still one fsync per request.
- No group commit in this entry.

## Auth
- HS256 JWT in `Authorization: Bearer <token>`, secret from `JWT_SECRET`
- Required on `POST /posts`, `POST /posts/:id/like`, and `POST /posts/:id/comments`
- The username comes from the token's `sub`

## Notes
- Each simulated user loop has two durable writes: the like or comment, and the profile visit. That is why the score tops out near the disk's synced-write ceiling, not CPU.
