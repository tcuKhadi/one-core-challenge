# one-core-challenge

How many simulated users can one server handle on **1 CPU core and 2 GB RAM, no swap**?

Everyone runs the same seven-endpoint app on the same box. The score is the highest load level that holds for 5 minutes without breaking the pass rules. See [rules.md](rules.md) and [api.md](api.md).

## How to enter

1. Implement the app from [api.md](api.md), storage is in-process SQLite only.
2. Make your entry folder: `entries/<your-github-username>/` with a `Dockerfile`. The Dockerfile *is* the start command - the app listens on port 8080, containerized alone.
3. Submit one pull request containing only your entry folder.
4. CI runs a spec check against your container: all seven endpoints, status codes, JSON shapes, then a `kill -9` durability spot check ([spec-check.sh](spec-check.sh), you can run it locally too).
5. Passing entries get load tested on a reference machine with one published k6 script at +50 user steps. CI checks conformance. It never computes your score.

By opening a PR you accept [rules.md](rules.md) - including the parts people usually argue about: every write is fsynced before the reply is sent, every run starts cold (`kill -9` must not lose data), loads are uniform, and any code that detects load-test traffic is a void run.
