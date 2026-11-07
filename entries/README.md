# entries/

One folder per entry: `entries/<your-github-username>/`.

- Required file: a `Dockerfile` at the folder root. It is the start command.
- The container serves all seven endpoints on port 8080 ([api.md](../api.md)).
- The app reads `JWT_SECRET` from the environment. The standard run command sets it ([rules.md](../rules.md)).
- Everything is inside the image. No host mounts, no sidecar containers, no network calls, no pre-baked database.

The build happens with `docker build -t entry entries/<your-username>/` and the run with the standard command in [rules.md](../rules.md).
