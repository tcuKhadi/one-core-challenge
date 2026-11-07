Submission Checklist

Please verify each item before opening your pull request:
[ ] All submission files are contained strictly inside entries/<your-github-username>/.
[ ] No files outside your folder (like root config, rules, or test harnesses) have been modified.
[ ] Your entry includes a working Dockerfile that builds and starts the application listening on port 8080.
[ ] Reads the JWT signing secret directly from the JWT_SECRET environment variable.
[ ] Uses in-process SQLite with a single database file residing on the container's own filesystem (no /dev/shm, tmpfs, or memory mode).
[ ] Configured with synchronous = FULL or EXTRA, and journal_mode = WAL | DELETE | TRUNCATE | PERSIST on every connection.
[ ] Every write (posts, comments, first likes, and profile visits) is committed and synced to disk before the HTTP response is sent.
[ ] Local verification passes: ./spec-check.sh <container-name> exits with 0 failures, including the kill -9 durability crash test.

Stack Details
Language / Runtime:

SQLite Driver / Library:

Concurrency / Architecture Notes (Optional):
