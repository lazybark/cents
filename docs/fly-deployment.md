# Deploying cents, its Telegram bot and Mini App on Fly.io

`cents-cloud` starts the existing API server, Telegram bot and a reverse proxy
in one Go process on one Fly Machine. The standalone server and bot commands
remain available. The desktop app runs on your computer and connects remotely.

```text
                         one Fly Machine
HTTPS -> Fly TLS -> gateway :8080
                       |-- /app/* -> bot's Mini App, 127.0.0.1:8750
                       |              (strips /app; keeps Telegram authentication)
                       |-- /api/* -> API, 127.0.0.1:8740 (bearer authentication)
                       |-- /healthz -> checks both HTTP listeners
                       `-- / -> /app/

Telegram <-- long polling -- bot -- localhost HTTP --> API --> /data/cents.db
                                  all components' logs --> /data/logs.db + stdout
                                  reminder state --> /data/bot-state.json
```

The Mini App URL must end in `/app/`, including the trailing slash. Its assets
and API calls use relative URLs, so they stay under that prefix. The desktop
connects to the root address, such as `https://YOUR_APP.fly.dev`.

## Lifecycle and configuration

Build with `make build-cloud`. To run outside Docker:

```sh
bin/cents-cloud config example
bin/cents-cloud --config /path/to/cloud.yml
```

The gateway uses `cloud.yml`, plus the existing `server.yml` and `bot.yml`.
Missing config keys receive defaults; unknown keys are rejected. The API and
Mini App listeners must use loopback IPs. The bot's `server.url` must match
the API listener, and its token must match a writable server token.

The server starts before the bot uses it. The gateway's readiness check
returns 503 until both upstream HTTP endpoints respond. A fatal service exit
cancels its siblings, waits for shutdown, flushes queued logs and exits with
a failure status. Fly's restart policy then restarts the bundle. Temporary
Telegram polling failures still use the bot's existing retry loop.

Normal SIGINT/SIGTERM shuts everything down together. Readiness checks don't
test Telegram's external availability, and an unhealthy check alone does not
trigger a restart. Deployments briefly interrupt service. Keep exactly one
Machine to avoid concurrent Telegram pollers and independent budget copies.

## Prepare the configs

The `deploy/*.yaml.example` files are templates; actual YAML files and
`.deploy/` are ignored by Git and the Docker build context.

```sh
mkdir -p .deploy
cp deploy/server.yaml.example .deploy/server.yml
cp deploy/bot.yaml.example .deploy/bot.yml
make build-server
bin/cents-server token new --name bot
bin/cents-server token new --name desktop
```

Put the two generated SHA-256 hashes in `.deploy/server.yml`. Keep the raw
desktop token for the desktop app. The raw bot token becomes
`CENTS_BOT_SERVER_TOKEN`; the Telegram token from BotFather becomes
`CENTS_BOT_TELEGRAM_TOKEN`. Those two environment variables are already
supported by the bot, so the token fields in `bot.yml` can stay empty.

In `.deploy/bot.yml`, set your Telegram user ID and the public Mini App URL,
`https://YOUR_APP.fly.dev/app/`. Leave the internal server URL at
`http://127.0.0.1:8740` and the Mini App listener at `127.0.0.1:8750`.
The bot sets its Telegram menu button to that URL at startup.

`deploy/cloud.yaml.example` is the gateway config copied into the image as
`/etc/cents/cloud.yml`. By default it creates an empty, unencrypted budget if
`/data/cents.db` doesn't exist. Existing files are never replaced. Set
`initialize_database: false` if missing budget data should prevent startup.
An empty budget needs its currencies set up through a client before use.
Password-encrypted budget files are not supported by the API server yet.

## Deploy

Set a unique app name in `fly.toml`. Its region defaults to `waw`; change it
and `TZ = "Europe/Kyiv"` if appropriate. Timezone data is embedded in the
cloud binary. The Docker build enables CGO for the SQLite driver.

Create the app and its volume, using the same name and region as `fly.toml`:

```sh
fly apps create YOUR_APP
fly ips allocate-v4 --shared
fly ips allocate-v6
fly volumes create cents_data --region waw --size 1
```

Stage the configuration as file secrets. Fly decodes these base64 values
into `/etc/cents/server.yml` and `/etc/cents/bot.yml` at deployment:

```sh
fly secrets set --stage \
  CENTS_SERVER_CONFIG="$(base64 < .deploy/server.yml | tr -d '\n')" \
  CENTS_BOT_CONFIG="$(base64 < .deploy/bot.yml | tr -d '\n')"
fly secrets set --stage \
  CENTS_BOT_TELEGRAM_TOKEN='YOUR_TELEGRAM_BOT_TOKEN' \
  CENTS_BOT_SERVER_TOKEN='YOUR_RAW_CENTS_BOT_TOKEN'
fly deploy --ha=false
fly scale count 1
fly checks list
```

Use `fly deploy --ha=false` for subsequent deployments too. The provided
configuration disables autostop and uses rolling replacement on the single
Machine. It exposes only port 8080 through Fly's HTTPS proxy. Fly terminates
TLS; the gateway and internal services communicate over HTTP on this Machine.

`cents-cloud` configures the API to trust forwarding information only from
its loopback gateway. The gateway accepts `Fly-Client-IP` only from peers in
`cloud.yml`'s `trusted_proxies` list, overwrites it on upstream requests, and
ignores caller-supplied forwarding chains. This preserves the API allowlist
and separate failed-login limits for external clients. For local use, set
the gateway's `trusted_proxies: []`.

The sample API allowlist permits all client IPs; a valid token is still
required. You can narrow it to your client networks, retaining loopback for
the bot and internal health probes. Telegram Mini App access uses the bot's
allowed Telegram users and signed Telegram initData separately.

For an existing budget, first let a new deployment start with an empty file.
Use `fly machine list` to find its ID, then temporarily run a maintenance
command instead of cents. This keeps SFTP available with the budget closed:

```sh
fly machine update MACHINE_ID --command "sleep infinity" --skip-health-checks
fly ssh sftp shell
```

In the SFTP shell, upload a consistent export/backup from cents:

```text
put /path/to/your-backup.db /data/cents.db
exit
```

Resume the bundle with the same config:

```sh
fly machine update MACHINE_ID --command "cents-cloud --config /etc/cents/cloud.yml"
```

Never replace an open budget database. Keep a local backup before migration;
do not copy an actively written SQLite file. The deployment does not
implement replication or automatic off-Machine backups.

## Inspect logs

`logs.db` is a separate SQLite database on the same persistent `/data` volume
as `cents.db`. Its schema is independent of the budget. Records include UTC
timestamp, severity, component (`cloud`, `server`, `bot`, or `gateway`), message
and JSON attributes. The gateway records method, client category, status,
duration, response bytes, original client IP and a generated request ID,
also returned in `X-Request-ID`.

Client categories include `mini_app_api`, `desktop_api` and `admin_api`;
older events stored this category under `route`. The admin app exposes client
and HTTP status as separate columns and filters. Mini App API requests also
produce bot request logs, while server requests retain the authenticated
token name in `client`.

Request bodies, query strings, authorization headers and Telegram initData
are not included in gateway access logs. Known bot/server secrets are
redacted before stdout and SQLite output, including tokens embedded in
Telegram error URLs. Existing API and bot lifecycle/error logs are captured
as JSON too. The separate [desktop admin app](admin.md) reads these logs
through the API using an admin token. Ordinary desktop/bot tokens and the
Telegram Mini App cannot access that endpoint.

The default retention is 30 days, pruned on startup and hourly. Set
`log_retention_days: 0` to retain all events. SQLite reuses deleted pages;
pruning doesn't shrink the file. WAL mode allows concurrent inspection.
Writes are queued and shutdown flushes for up to ten seconds; a full queue,
flush timeout or disk failure loses database log entries
while stdout continues and reports the logging failure. This is diagnostic
logging, not a transactional financial audit trail. A volume persists across
deployments; it is not an independent backup.

Stream stdout:

```sh
fly logs
```

Inspect recent errors with the `sqlite3` CLI included in the image:

```sh
fly ssh console
sqlite3 -readonly /data/logs.db
```

```sql
.headers on
.mode column
SELECT id, timestamp, level, component, message
FROM logs
ORDER BY id DESC
LIMIT 100;

SELECT timestamp, component, message, attributes
FROM logs
WHERE level IN ('WARN', 'ERROR')
ORDER BY id DESC
LIMIT 100;

SELECT timestamp, json_extract(attributes, '$.status') AS status,
       json_extract(attributes, '$.duration_ms') AS duration_ms
FROM logs
WHERE component = 'gateway'
ORDER BY id DESC
LIMIT 100;
```

To download a consistent log database while the app runs, use SQLite's backup
command, then download `/tmp/cents-logs.db` with Fly SFTP. Don't copy only the
live `logs.db` file: recent records can still be in its WAL.

```sh
sqlite3 /data/logs.db ".backup '/tmp/cents-logs.db'"
```

Fly references: [configuration and file secrets](https://docs.fly.io/reference/configuration),
[single-Machine deployment](https://docs.fly.io/apps/app-availability), and
[persistent volumes](https://docs.fly.io/volumes/overview).
