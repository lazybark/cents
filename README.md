# cents

A desktop budget app with a Go API server, Telegram bot and Telegram Mini App.

For a single Fly.io Machine, `make build-cloud` builds `cents-cloud`, which
runs the server, bot and public gateway together and stores diagnostic logs
in a separate persistent SQLite database. See [deployment and log inspection](docs/fly-deployment.md)
for configuration, tokens, Docker and Fly setup.

Run `make build-docker` to build or rebuild the local server image (`cents-cloud:local`).

Run `make run-admin` to build and open the separate desktop administration app.
Its **Logs** screen connects to the server with an admin token and reads the
persistent diagnostic logs. See [admin setup and usage](docs/admin.md).

The desktop's **Backup & export** screen can [merge records from another cents database](docs/importing.md)
into a local budget or connected server, with a preview and duplicate detection.

