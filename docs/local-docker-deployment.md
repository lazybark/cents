**The current image needs YAML configs plus two environment variables for the tokens.** You can mount the configs and keep both databases in a Docker volume.

1. Build and copy the templates:

```sh
docker build -t cents-cloud:local .

mkdir -p .deploy
cp deploy/cloud.yaml.example .deploy/cloud.yml
cp deploy/server.yaml.example .deploy/server.yml
cp deploy/bot.yaml.example .deploy/bot.yml
```

2. Generate API tokens:

```sh
docker run --rm cents-cloud:local cents-server token new --name bot
docker run --rm cents-cloud:local cents-server token new --name desktop
```

Put the generated **hashes** into `.deploy/server.yml`. Keep the raw desktop token for connecting your desktop app.

3. Edit the local configs:

- In `.deploy/cloud.yml`, set `trusted_proxies: []`.
- In `.deploy/bot.yml`, set your Telegram user ID.
- Set `web_app.url` to your public HTTPS tunnel address, ending in `/app/`, for example `https://your-tunnel.example/app/`.
- Point that tunnel at **`http://localhost:8080`**, so requests reach the gateway.

Keep the internal addresses at `127.0.0.1:8740` and `127.0.0.1:8750`; both services run inside the container.

4. Create `.deploy/local.env`:

```dotenv
CENTS_BOT_TELEGRAM_TOKEN=YOUR_BOTFATHER_TOKEN
CENTS_BOT_SERVER_TOKEN=YOUR_RAW_BOT_API_TOKEN
```

The second value must match the bot token whose hash you added to `server.yml`.

5. Run it:

```sh
docker run --rm \
  --name cents-local \
  --stop-timeout 120 \
  -p 127.0.0.1:8080:8080 \
  --env-file .deploy/local.env \
  -e TZ=Europe/Kyiv \
  --mount "type=bind,src=$(pwd)/.deploy,dst=/etc/cents,readonly" \
  --mount type=volume,src=cents-local-data,dst=/data \
  cents-cloud:local
```

This starts an empty budget on first run. The `cents-local-data` volume preserves `cents.db`, `logs.db`, and reminder state across container removal.

Check readiness with:

```sh
curl http://localhost:8080/healthz
```

Then open the Mini App through Telegram. Stop your currently running bot first if you reuse its Telegram token.