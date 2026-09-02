# Sol

Sol is a monorepo for the home-automation platform. It manages homes, rooms, appliances and devices; sends device commands through MQTT; stores telemetry; builds and distributes ESP32 firmware; and provides automation, chat, MCP, and voice-session integrations.

The machine-readable HTTP reference is [apps/core/api/openapi.yaml](apps/core/api/openapi.yaml).

## Architecture

```text
Sol frontend / MCP clients ──HTTP + WebSocket──> sol-core
                                               ├── PostgreSQL + TimescaleDB
                                               ├── Redis
                                               ├── MinIO (firmware artifacts)
                                               └── MQTT broker (mTLS)

ESP32 devices ────────────────────────────────MQTT──┘
Firmware build requests ──Redis queue──> sol-builder ──> ESP-IDF
Voice sessions ──> LiveKit + sol-agent
```

## What is in this repository

| Path | Purpose |
| --- | --- |
| `apps/core` | Go API server, OpenAPI spec, and Goose migrations |
| `apps/web` | Next.js web client |
| `apps/builder` | Redis-backed ESP-IDF firmware-build worker |
| `apps/agent` | LiveKit voice agent (`Joy`) |
| `packages/firmware` | ESP-IDF firmware source and components |
| `infra/` | Dockerfiles and Traefik configuration |
| `docker-compose.yml` | Root local-development and deployment stack |

## Prerequisites

- Go **1.26**
- Docker Engine with Docker Compose
- An OIDC/Zitadel issuer for production authentication

Optional capabilities also need their respective credentials: Azure/Kimi and Cohere for AI features, Brevo for invitation email, and LiveKit plus Azure OpenAI Realtime for voice sessions.

## Local development

`docker compose up --build` starts a usable local stack with PostgreSQL, Redis, MinIO, VerneMQ, Goose migrations, Sol Core, the firmware builder, and Sol Next. It generates a development CA and uses a local-development account, so no external credentials are required for the default path.

To customize values, copy the checked-in environment template. `.env` overrides the template and is ignored by Git; never commit credentials.

```bash
cp .env.example .env
```

```bash
docker compose up --build
```

Open `http://localhost:3000`, select **Log in / Sign up**, and use the generated local account. The API is available at `http://localhost:8080`.

### Adopting an existing database

Databases created with the retired migration script have no Goose history. Do this once **before** running the new migration service, using the last migration that is already present in that database. For a database maintained by the previous script, that version is `17`:

```bash
./scripts/adopt-goose.sh 17
./scripts/migrate.sh
```

The adoption script refuses to run if Goose has already been initialized. Confirm the version from your deployment history before using a value other than `17`.

The development server listens on `http://localhost:8080`. Confirm it is running:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

The Compose stack provides a development VerneMQ broker at `tcp://vernemq:1883`. It allows anonymous TCP connections and is suitable only for local development. Production should use an `ssl://` or `tls://` broker URL; Sol Core then requires mTLS and validates the broker against `CA_CERT_PATH`.

### Full Compose stack

The default command intentionally excludes externally dependent services. Add the `voice` profile after configuring LiveKit and Azure credentials; add the `edge` profile for Traefik and public TLS after configuring `API_DOMAIN`, `ACME_EMAIL`, a real OIDC issuer, and production certificate material:

```bash
docker compose --profile voice --profile edge up --build
```

Traefik exposes ports 80 and 443. It forwards requests for `API_DOMAIN` to `sol-core`.

## Configuration

The API reads configuration from environment variables. Defaults are intended for local development where noted.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listener port |
| `DATABASE_URL` | local `sol` PostgreSQL URL | PostgreSQL/TimescaleDB connection |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection and build queue |
| `MQTT_BROKER_URL` | configured Sol broker URL | MQTT broker URL (`ssl://` for mTLS) |
| `MQTT_CLIENT_ID` | `sol-backend` | Backend MQTT client ID |
| `CA_CERT_PATH` / `CA_KEY_PATH` | — | CA certificate and private key used to issue device/client certificates; both are required |
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO/S3 endpoint |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | `minioadmin` | Object-storage credentials |
| `MINIO_USE_SSL` | `false` | Enable TLS for MinIO when set to `true` |
| `MINIO_BUCKET` | `firmware` | Firmware artifact bucket; created at startup if absent |
| `OIDC_ISSUER` | — | Zitadel/OIDC issuer; used for userinfo lookups and Traefik ForwardAuth |
| `INTERNAL_SERVICE_TOKEN` | — | Required bearer token for internal builder and voice-agent calls |
| `FRONTEND_URL` | `http://localhost:3000` | Base URL used in invitation emails |
| `PUBLIC_API_URL` | `http://localhost:8080` | API URL included in OTA flows |
| `OTA_API_URL` | configured Sol OTA URL | mTLS-protected OTA host used by devices |
| `OTA_ONLINE_FRESHNESS_SEC` | `45` | Maximum age of a device heartbeat for OTA eligibility |
| `OTA_ATTEMPT_TIMEOUT_SEC` | `480` | OTA-attempt timeout |
| `AI_SERVICE_URL` | `http://localhost:8000` | Automation AI service endpoint |
| `KIMI_ENDPOINT`, `KIMI_API_KEY`, `KIMI_DEPLOYMENT` | —, —, `Kimi-K2.6` | Chat model configuration |
| `COHERE_AZURE_ENDPOINT`, `COHERE_AZURE_KEY`, `COHERE_AZURE_DEPLOYMENT`, `COHERE_API_VERSION` | deployment defaults | Appliance embeddings configuration |
| `BREVO_API_KEY`, `BREVO_SENDER_EMAIL`, `BREVO_SENDER_NAME` | email disabled without key | Invitation email delivery |
| `LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET` | service URL / empty keys | Voice-session management |

## API and authentication

Use [apps/core/api/openapi.yaml](apps/core/api/openapi.yaml) for documented HTTP schemas; it is suitable for Swagger UI, Redoc, Postman, and code-generation tools. The route registration in `apps/core/cmd/sol/main.go` is the definitive runtime surface while the specification is brought fully in sync.

The API surface includes:

- homes, members, invitations, per-room member permissions, and ownership transfer;
- rooms, activity history, devices, appliances, telemetry, and live device commands;
- automations, firmware uploads/builds/versions, and OTA attempt management;
- WebSocket updates at `/ws`, chat WebSocket sessions, and MCP-over-SSE;
- device provisioning and certificate issuance, plus public and mTLS-protected OTA downloads.

Except for the health endpoint, selected invitation endpoints, and OTA endpoints, routes require an OIDC access token:

```http
Authorization: Bearer <access_token>
```

In the Compose deployment, Traefik validates API bearer tokens with the issuer’s `/oidc/v1/userinfo` endpoint before forwarding requests. The application also derives the current Sol user from token claims. WebSocket clients may provide the token as `?token=` because browsers cannot set arbitrary headers during a WebSocket upgrade.

## Firmware, builds, and voice

`sol-builder` waits on the Redis `firmware_build_queue`, builds the ESP-IDF project in `packages/firmware/`, streams build logs to internal API endpoints, and stores produced artifacts through the core API. It uses `INTERNAL_SERVICE_TOKEN` for those internal requests.

`sol-agent` is a LiveKit worker that powers the Joy voice assistant. It calls the internal voice context and tool-dispatch endpoints exposed by `sol-core` using `INTERNAL_SERVICE_TOKEN`. Configure the required LiveKit and Azure OpenAI environment variables before enabling it.

## Verify changes

Run the Go tests and build before opening a pull request:

```bash
go -C apps/core test ./...
go -C apps/core build ./cmd/sol
```

For the firmware worker:

```bash
go -C apps/builder build ./...
```

## Notes for contributors

- Keep [apps/core/api/openapi.yaml](apps/core/api/openapi.yaml) aligned with changes to public HTTP handlers.
- Add schema changes in `apps/core/migrations/` as a new numbered Goose migration rather than modifying an applied migration. Use `docker compose run --rm migrate -s create <name> sql`, then run `./scripts/migrate.sh` and `./scripts/migrate.sh status` to verify it. Existing databases must be adopted once with `./scripts/adopt-goose.sh <version>` before their first Goose run.
- Do not commit `.env`, certificate material, generated ESP-IDF build output, or firmware dependency lock/build artifacts.
