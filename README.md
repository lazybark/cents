# cents

A desktop budget app with a Go API server, Telegram bot and Telegram Mini App.

For a single Fly.io Machine, `make build-cloud` builds `cents-cloud`, which
runs the server, bot and public gateway together and stores diagnostic logs
in a separate persistent SQLite database. See [deployment and log inspection](docs/fly-deployment.md)
for configuration, tokens, Docker and Fly setup.

