# Polaris

Polaris is a self-hosted campaign and character tracker for writers, game
masters and worldbuilders. Keep your characters (with versions over time),
their stats, gear and spells, a hex map, a calendar, events and a timeline
for each of your stories, in one place that you own.

- **Characters** with a version history, a stat build that is computed for
  you, a paper-doll gear tab and a spell list.
- **Character Assets**: classes, subclasses, specializations, races, body
  types, gear and spells, each able to add stat bonuses.
- **Map**: a hex map with major locations you can paint and inspect.
- **Calendar, Events and Timeline**: your own calendar (months per year, days
  per month), events linked by tags and a timeline drawn in proportion to time.
- **Multiple stories**, each fully separate, with export and import as a zip.
- **Languages**: English, Português (Brasil), Español (España) and Español
  (Latinoamérica). Only the interface is translated, never what you write.
- Five colour themes.

It is a single Go program (with the web app built in) and a SQLite database
per story, so it runs happily on a small home server.

> **Provided as is.** Polaris was built for one person's own use and shared
> in case it is useful to someone else. There is no support and no promise of
> updates. See [LICENSE](LICENSE).

## Quick start (Docker)

You need Docker with the Compose plugin.

    git clone <this repository's URL> polaris
    cd polaris
    docker compose up -d --build

Then open <http://localhost:8091> and create your first story.

## Security: read this before exposing it

**Polaris has no login.** Anyone who can reach the page can read and change
everything. That is why the default setup only listens on `127.0.0.1`, which
means only the machine it runs on can open it.

To use it from other devices, put your own protection in front of it. Common
choices are a reverse proxy that requires a login, a VPN such as Tailscale or
WireGuard, or a tunnel with access control such as Cloudflare Tunnel with
Cloudflare Access. Then let it listen on your network by setting
`BIND_ADDR` (below). Do not forward its port straight to the internet.

## Configuration

Copy `.env.example` to `.env` and edit it. Compose reads it automatically,
and git ignores it, so your settings survive updates.

| Variable    | Default     | Meaning                                                    |
|-------------|-------------|------------------------------------------------------------|
| `BIND_ADDR` | `127.0.0.1` | Address the app listens on. `0.0.0.0` = your whole network |
| `HOST_PORT` | `8091`      | Port on the host                                           |

Inside the container the app also reads `PORT`, `DATA_DIR` and `UPLOADS_DIR`
(already set in `docker-compose.yml`).

## Your data and backups

Everything lives in two Docker named volumes, `polaris_data` (the databases)
and `polaris_uploads` (pictures and icons). Rebuilding or updating the
container never touches them.

The easiest backup is built in: on the story picker, **Export** a story to
a `.zip` (its database and pictures). **Import** turns a zip back into a new
story, so it also works for moving to another machine. Do this regularly, and
before every update.

To back up the raw volumes instead:

    docker run --rm -v polaris_data:/d -v "$PWD":/b alpine tar czf /b/polaris_data.tgz -C /d .
    docker run --rm -v polaris_uploads:/d -v "$PWD":/b alpine tar czf /b/polaris_uploads.tgz -C /d .

## Updating

    git pull
    docker compose up -d --build

(`scripts/update.sh` does exactly this.) Databases upgrade themselves on
start-up and keep all your data. Upgrades only go forward: a database that
has been upgraded cannot be opened by an older version, so back up first.

## Running without Docker

You need Go 1.22+ and Node 20+.

    cd frontend && npm ci && npm run build
    cp -r dist/. ../backend/static/
    cd ../backend
    go build -o polaris .
    DATA_DIR=./data UPLOADS_DIR=./data/uploads ./polaris

It listens on port 8091 (`PORT` changes it) and on all interfaces, so run it
behind something that restricts access if the machine is on a network.

## Notes

- The interface loads its fonts from Google Fonts. Your browser contacts
  Google for them; there is no other external request.
- Contributing and the technical notes are in
  [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).
