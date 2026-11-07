# one-core-challenge

How many simulated users can one server handle on **1 CPU core and 2 GB RAM, no swap**?

Everyone runs the same seven-endpoint app on the same box. The score is the highest load level that holds for 5 minutes without breaking the pass rule. See [rules.md](rules.md) and [api.md](api.md).

## How to enter

1. Read [api.md](api.md) and the SQLite section of [rules.md](rules.md).
2. Fork this repo.
3. Add your entry folder: `entries/<your-github-username>/` with a `Dockerfile`. The Dockerfile is the start command. The app listens on port 8080 and reads `JWT_SECRET` from the environment.
4. Open a pull request that contains only your entry folder.
5. That is all. Your entry gets the spec check, the durability check, and the load test on the reference machine, and the results get published.

You can run [spec-check.sh](spec-check.sh) against your container yourself before you open the pull request.

By opening a PR you accept [rules.md](rules.md), including the parts people usually argue about: every write is synced to disk before the reply is sent, every run starts cold and is seeded the same way, the loads are the same for everyone, and any code that detects load-test traffic is a void run.
