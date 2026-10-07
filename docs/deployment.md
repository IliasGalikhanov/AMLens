# Deployment and operations

## Local Docker installation

Use Linux Docker Engine with Compose v2 or Docker Desktop with Linux containers. The public demo override requires Compose **2.24.4+**. From the root:

```sh
docker compose up --build -d --wait
docker compose ps
docker compose logs --tail 100
```

Website: http://localhost:8080. Health: http://localhost:8080/api/health. The backend has no separate public host port. Docker Desktop must be running on Windows.

If the port is occupied, copy `.env.example` to the root `.env` and change `APP_PORT`. AI is optional: configure `OPENAI_API_KEY` and `OPENAI_MODEL` there. After changing environment variables, run `docker compose up -d`; a restart alone does not reload Compose environment settings. Do not share `docker compose config` output containing secrets.

Direct Go execution reads a different file, `backend/.env`, only with `--env-file .env`. Never use `VITE_*` for secrets. Container images do not include `.env` or user data.

## Synthetic demo

```sh
docker compose -f compose.demo.yaml up --build -d --wait
```

Open http://localhost:8081. This uses a separate `amlens-demo` project and `demo-analyses` volume. Configure `DEMO_PORT` in the root `.env`. Do not add `compose.yaml` to this command: regular and demo modes have independent definitions.

## Public demo with HTTPS

On a Linux server, install Docker/Compose, copy the source, point the domain's A/AAAA records to the server and allow inbound TCP 80/443. UDP 443 is optional for HTTP/3. Set `DOMAIN` in the root `.env`, for example `demo.example.org`, without a scheme or path.

```sh
docker compose -f compose.demo.yaml -f compose.public-demo.yaml up --build -d --wait
```

The site will be available at your HTTPS domain. Caddy requests and renews certificates automatically; DNS and certificate authority validation must reach this server. Certificates are stored in separate volumes. Local configuration validation does not demonstrate successful issuance of a real certificate.

Use this override only for the synthetic demo. Keep private analysis local or behind a VPN because the application does not implement user authentication. English is the initial interface language; visitors can select Russian or Kazakh without server configuration.

## Updates and rollback

Before updating, back up the database and record the current source revision. Then:

```sh
docker compose build --pull
docker compose up -d --wait
docker compose logs --tail 100 backend
```

For demo deployments, use the same `-f` files as at startup. Base image branches are Go 1.25, Node 24, Caddy 2 and Alpine 3.22; `--pull` picks up patch releases. `go.sum` and `package-lock.json` pin application dependencies. Record built image digests for an immutable deployment.

To roll back, restore the previous source and a compatible database backup, rebuild and start. Check compatibility before opening a newer database with an older application.

## Backups

Ordinary `docker compose down` preserves the volume; `down --volumes` deletes it. SQLite contains the graph, metrics, explanations and CSV, but not original Parquet files. History has no automatic retention limit; monitor volume size.

For a simple backup with the API stopped, run from the root:

```sh
docker compose stop backend
docker compose cp backend:/var/lib/amlens/analyses.db ./analyses-backup.db
docker compose start backend
```

Back up before updating and keep the backup outside the public repository. To restore, stop the backend and separately preserve its current database, then:

```sh
docker compose cp ./analyses-backup.db backend:/var/lib/amlens/analyses.db
docker compose run --rm --no-deps --user 0 --cap-add CHOWN --entrypoint chown backend 10001:10001 /var/lib/amlens/analyses.db
docker compose start backend
```

Do not use ordinary file copying on a live SQLite database: stop the API first to finish transactions. Verify health, history, a client card and CSV after restoring. A separate Compose project can be used to validate backups.

## Diagnostics

Health reports server readiness and the presence of an active analysis. `ai_configured` confirms configuration, not external provider availability. Read logs with `docker compose logs --tail 100 backend frontend`. Logs do not include API keys or uploaded table bodies. A database opening/decoding failure stops startup; the application does not silently reset the database.

For an unhealthy container, check logs, free disk space and Docker availability. Do not delete the volume as a repair shortcut. For HTTP 413, reduce the input size; for 422, correct the indicated field/row; for 409, wait for the active operation and refresh. If the global graph is slow, explore a client's neighborhood.
