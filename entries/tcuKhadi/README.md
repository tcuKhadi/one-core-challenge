# tcuKhadi reference solution

Go + Gin + SQLite (in-process). This is the reference entry, not a limit-pusher.

## Storage approach
- Single SQLite file `challenge.db` in the container working directory
- `journal_mode = WAL`, `synchronous = FULL` — every completed write is fsynced before the reply
- One pooled connection (`MaxOpenConns = 1`): commits serialize, behavior is deterministic. Reads queue behind writes at high load — fine for a reference.

## Notes for the video
- The durability rule is the whole game: ~2 fsynced writes per simulated user per loop tops the score out near the disk's synced-write ceiling, not CPU
