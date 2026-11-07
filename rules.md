# Rules

## The box
- One CPU core, pinned. 2 GB RAM, no swap.
- Same container command for every entry:
  `docker run -d --name challenge --cpuset-cpus=0 --memory=2g --memory-swap=2g -e JWT_SECRET=one-core-challenge-secret -p 8080:8080 <image>`
  (cleaned up with `docker rm -f challenge`).
- No kernel tuning, no privileged flags, no extra processes.

## The app
- Every entry implements the same seven endpoints: [api.md](api.md).
- One process inside the container. SQLite in-process is the only storage. No separate database, cache, or queue, and no network calls.
- The app reads the token secret from the `JWT_SECRET` environment variable (default for benchmarking: `one-core-challenge-secret`).

## SQLite rules
These rules exist so every entry pays the same price for durability.

**Required**
- A real SQLite library, in-process. Any language, any driver. No other embedded database, no SQLite-compatible server.
- One database file, on the container's own filesystem. No RAM-backed paths (`/dev/shm`, tmpfs) and no in-memory databases (`:memory:`, `mode=memory`).
- `synchronous` set to `FULL` or `EXTRA` on every connection, for the whole run.
- `journal_mode` is `WAL`, `DELETE`, `TRUNCATE`, or `PERSIST`. Never `OFF` or `MEMORY`.
- The app creates the file and the schema itself on first start, from nothing. Schema creation must be safe to run again on restart.
- Every write (posts, comments, first likes, and profile visits) is committed and synced to disk before its HTTP reply is sent.
- A read that starts after a write's reply must see that write.

**Allowed**
- Any schema and any indexes. The schema is yours.
- Prepared statements, any connection pool size, any number of reader connections.
- `cache_size`, `mmap_size`, `temp_store`, `busy_timeout`, and checkpoint settings. Memory they use counts toward the 2 GB.
- Group commit: several requests may share one transaction and one sync, as long as no request gets its reply before the transaction containing it is committed and synced.
- Caching data that is already committed.

**Not allowed**
- Lowering `synchronous` or changing `journal_mode` to an unsafe mode at any point.
- A custom VFS or any trick that skips the sync.
- Holding writes in memory and flushing them later while clients have already been told "done" (write-behind).
- A second database file, a read-only copy, or any storage outside the one SQLite file.

## Seed data
Every run starts with the same data, loaded through the public API by [seed.js](seed.js) after the container starts.

| Data | Amount |
| --- | --- |
| Users | 1,000 (`user0000` to `user0999`) |
| Posts | 10,000 |
| Comments | 30,000 (3 per post) |
| Likes | 100,000 (10 per post, from 10 different users) |

- The seed is about 140,000 writes. It is not timed, and it must finish with zero errors.
- Seed requests are ordinary requests. Code that treats them differently is a void run.
- Entries do not ship data. No pre-baked database in the image.

## Simulated users
- One simulated user runs a loop: `GET /feed` -> `GET /posts/:id` -> one write (`POST /posts/:id/like` or `POST /posts/:id/comments`) -> `GET /users/:username`, pausing 2–5 s between actions.
- Everyone uses the same load script ([load-test.js](load-test.js)).
- Simulated user count includes only users actually running the loop. No idle connections inflating the score.

## Pass rule
**Levels.** A level is a fixed number of simulated users, always a multiple of 50.

**One level run**
1. Remove and recreate the container with the standard command.
2. Load the seed data via `seed.js`.
3. Ramp up to the level over 60 seconds. This part is not measured.
4. Hold the level for 5 minutes. Only this part is measured.

**A level passes if, over the 5-minute hold:**
- Errors are under 1% of all requests. These count as errors: a request with no reply within 5 seconds, a connection reset or refused, any 5xx, and any reply that breaks [api.md](api.md) (wrong status, invalid JSON, missing fields).
- 95% of replies come back in under 500 ms, measured across all requests from all endpoints together.
- The load was really applied. The number of active users stayed at the level, the load machine stayed under 80% CPU, and it never ran out of ports. If it did, the level is void and is run again.

**Finding the score.** Levels are tried in rising order: 100, 200, 400, and so on, doubling until a level fails. Then the search narrows between the last passing level and the first failing one, in steps of 50 users, until it finds a passing level whose next 50-user level fails. That passing level is the score. A level that fails is run once more, and it counts as failed only if it fails both times.

**Durability checks**
1. **Spec durability check:** Run by `spec-check.sh`. Kills the container process with `kill -9` after initial writes, restarts it, and asserts that all written data survived.
2. **Load durability check:** Once per entry, before final scoring, during a run at 200 users, the runner sends `kill -9` to the container process and restarts it. Every write that received an HTTP 201 before the crash must exist after restart, or the entry is disqualified.

## Disk budget
- At least 10 GB free on the server before any scoring run. Record `df` before and after each run.
- A run is void if free space drops below 2 GB during or after it.
- A scoring run adds tens of MB. Disk space is not the constraint, synced-write speed is.

## Honest behavior
- No hard-coded answers.
- No code that detects, throttles, or behaves differently for load-test traffic or seed traffic.
- Caching unchanged data (prepared statements, static parts of a reply) is fine. Serving stale data is not.
