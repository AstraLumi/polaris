# Polaris

Polaris is a self-hosted campaign and character tracker for writers, game
masters and worldbuilders. It keeps everything about a story in one place
that you own: characters and how they change over time, their stats, gear and
spells, a hex map, a custom calendar, events and a timeline.

Polaris is a single Go program with the web interface built in. Each story
has its own SQLite database, so it runs comfortably on a small home server.

> **Provided as is.** Polaris was built for personal use and is shared in case
> it is useful to others. There is no support and no commitment to future
> updates. See [LICENSE](LICENSE) (MIT).

## Features

- **Characters**: version history, automatically computed stats, a
  paper-doll gear tab and a spell list.
- **Character Assets**: classes, subclasses, specializations, races, body
  types, gear and spells, each of which can grant stat bonuses.
- **Map**: a paintable hex map with major locations you can inspect.
- **Calendar, events and timeline**: define your own calendar (months per year,
  days per month), link events with tags, and view them on a timeline drawn to
  scale.
- **Multiple stories**: each story is fully separate and can be exported to
  and imported from a `.zip` file.
- **Languages**: English, Português (Brasil), Español (España) and Español
  (Latinoamérica). Only the interface is translated, never your own content.
- **Themes**: five colour themes, chosen per device.

## Security

> [!WARNING]
> **Polaris has no login.** Anyone who can reach it can read and change
> everything.

For this reason the default setup listens only on `127.0.0.1`, so only the
machine running it can open it.

To use Polaris from other devices, put your own access control in front of
it, for example:

- a reverse proxy that requires authentication,
- a VPN such as Tailscale or WireGuard, or
- a tunnel with access control, such as Cloudflare Tunnel with Cloudflare
  Access.

Then set `BIND_ADDR` (see [Configuration](#configuration)) so it listens on
your network. **Do not forward its port directly to the internet.**

## Quick start (Docker)

Requirements: Docker with the Compose plugin.

```bash
git clone https://github.com/AstraLumi/polaris.git
cd polaris
docker compose up -d --build
```

Open <http://localhost:8091> and create your first story.

## Configuration

Copy `.env.example` to `.env` and edit it. Docker Compose reads it
automatically, and git ignores it, so your settings are kept across updates.

| Variable    | Default     | Description                                                       |
|-------------|-------------|-------------------------------------------------------------------|
| `BIND_ADDR` | `127.0.0.1` | Host address to listen on. `0.0.0.0` exposes it to your network.  |
| `HOST_PORT` | `8091`      | Port on the host.                                                 |

The application itself reads the following variables. `docker-compose.yml`
already sets them, so you only need them when
[running without Docker](#running-without-docker).

| Variable      | Default        | Description                                |
|---------------|----------------|--------------------------------------------|
| `PORT`        | `8091`         | Port the server listens on.                |
| `DATA_DIR`    | `/data`        | Folder for the story databases.            |
| `UPLOADS_DIR` | `/app/uploads` | Folder for uploaded pictures and icons.    |

## Data and backups

All data is stored in two Docker named volumes, which rebuilding or updating
the container never touches:

- `polaris_data`: the story databases
- `polaris_uploads`: uploaded pictures and icons

**Export (recommended).** On the story picker, **Export** saves a story,
including its database and pictures, as a `.zip`. **Import** restores a zip as
a new story, which also lets you move stories to another machine. Export
regularly, and always before updating.

**Raw volume backup.** To archive the volumes directly:

```bash
docker run --rm -v polaris_data:/d -v "$PWD":/b alpine tar czf /b/polaris_data.tgz -C /d .
docker run --rm -v polaris_uploads:/d -v "$PWD":/b alpine tar czf /b/polaris_uploads.tgz -C /d .
```

## Updating

Back up first (see above), then run:

```bash
./scripts/update.sh
```

The script pulls the latest code (fast-forward only), rebuilds and restarts
the container, and removes old images. Doing it by hand is equivalent to:

```bash
git pull
docker compose up -d --build
```

Databases are upgraded automatically on start-up and keep all their data.
Upgrades are one-way: once upgraded, a database cannot be opened by an older
version of Polaris.

## Running without Docker

Requirements: Go 1.26+ and Node.js 24+.

```bash
cd frontend && npm ci && npm run build
cp -r dist/. ../backend/static/
cd ../backend
go build -o polaris .
DATA_DIR=./data UPLOADS_DIR=./data/uploads ./polaris
```

The server listens on port 8091 on **all interfaces**. If the machine is on a
shared network, restrict access as described under [Security](#security).

## Privacy

The interface loads its fonts from Google Fonts, so your browser contacts
Google to fetch them. Polaris makes no other external requests.

## Development

Architecture notes and contribution guidelines are in
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).
