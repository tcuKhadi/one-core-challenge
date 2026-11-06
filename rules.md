# Rules

## The box
- One CPU core, pinned. 2 GB RAM, no swap.
- Same container command for every entry: `docker run -d --name challenge --cpuset-cpus=0 --memory=2g --memory-swap=2g -p 8080:8080 <image>` (cleaned up with `docker rm -f challenge`).
- No kernel tuning, no privileged flags, no extra processes.

## The app
- Every entry implements the same seven endpoints: [api.md](api.md)
- One process inside the container. SQLite in-process is the only storage. No separate database, cache, or queue, and no network calls.

## Simulated users
- One simulated user runs a loop: `GET /feed` -> `GET /posts/:id` -> one write (`POST /posts/:id/like` or `POST /posts/:id/comments`) -> `GET /users/:username`, pausing 2-5 s between actions.
- Everyone uses the same load script and the same ramp: fixed steps of +50 users.
- A level passes if it holds for 5 minutes with:
  - under 1% errors - timeouts, connection resets, and 5xx all count as errors
  - 95% of replies under 500 ms
- Score = the highest passing level.
- Simulated user count includes only users actually running the loop. No idle connections inflating the score.

## State and durability
- Every run starts cold: the container is removed and recreated, so the writable layer - and the database inside it - starts empty. No entry ships a pre-baked database in its image.
- SQLite runs with `synchronous=FULL`. Every write is saved to disk (fsynced) before the server replies.
- Spot check: during a run, the server process gets `kill -9` and the container is restarted. All writes must still exist, or the run is void.

## Disk budget
- At least 10 GB free on the server before any scoring run; record `df` before and after each run.
- A run is void if free space drops below 2 GB during or after it - a full disk invalidates results, and no entry or load behavior may rely on unbounded disk,
- Tracking: a scoring run adds single-digit MBs (a few hundred thousand small rows). Disk space is not the constraint; synced-write speed is.

## Honest behavior
- No hard-coded answers.
- No code that detects, throttles, or behaves differently for load-test traffic.
- A completed write must be visible to the next read (a liked post shows its new count).
- Caching unchanged data (prepared statements, static parts of a reply) is fine; serving stale data is not.
