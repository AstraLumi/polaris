# The API

A second port for programs, such as an LLM chat that reads your notes and
writes into your chapters. Off until `API_PORT` is set (README, "The API").
The code: `backend/apiserver.go` (the listener and its checks),
`backend/apikeys.go` (the keys and the log), `backend/apimcp.go` (the
tools, MCP and OpenAPI), `backend/apimarkdown.go` (chapters as Markdown).

## Keys

Made in **Settings → API access** on the app's own port; the API itself
can't make, list or revoke keys. A key looks like `pol_` and 43 more
characters. It is shown once; Polaris keeps only its SHA-256 (shown too,
for gateways that list keys that way). Each key:

- is **read only** or may **read and change**;
- reaches **every story** (new ones too) or only the ones picked;
- may **expire** at the end of a date (UTC);
- is revoked at once.

Send it with every request:

```
Authorization: Bearer pol_…
```

## What the port refuses

| Request | Answer |
|---|---|
| no key, an unknown, revoked or expired key | `401`, after a short wait |
| 20 wrong keys from one address in 10 minutes | `429` for that address (a valid key still works) |
| anything a browser sends (`Origin`, `Sec-Fetch-*`) | `403` |
| more than 600 requests a minute with one key | `429` |
| a body over 32 MB | `413` |
| a change with a read-only key | `403` (logged) |
| a story the key doesn't reach | `404` |

Every change (and every refused change) goes in the activity log, kept to
the last 1,000 and shown in Settings → API access. Before a story's first
change through the API each day, the server keeps a copy of it, as its
daily copies (Earlier copies in the story picker), when the story changed
since the last one. `AUTO_BACKUPS=off` turns that off too.

## Routes

| Route | What |
|---|---|
| `GET /` | the key's name and rights, and these routes |
| `GET /stories` | the stories the key reaches |
| `ANY /stories/{id}/{path}` | the story's own API, the same as the app's `/api/s/{id}/api/{path}`: every list, item, form and JSON body the app uses |
| `GET /uploads/s/{id}/{file}` | a picture, as answers name them |
| `POST /mcp` | Model Context Protocol |
| `GET /openapi.json` | the tools as OpenAPI 3.1 |
| `POST /tools/{name}` | one tool; the body is its arguments |

Through a gateway that forwards a path prefix, put that prefix in front of
each route (for example `https://gateway.example.com/v1/backend/mcp`).

## MCP

Streamable HTTP, without a server-sent stream or a session: every
`POST /mcp` gets its JSON-RPC answer in its own response (a notification
gets `202`). Methods: `initialize` (with the guide as `instructions`),
`ping`, `tools/list`, `tools/call`. Protocol versions 2025-06-18, 2025-03-26
and 2024-11-05.

A read-only key is never shown the tools that write.

| Tool | Writes | What |
|---|---|---|
| `list_stories` | | the stories the key reaches |
| `api_guide` | | how Polaris is organised, and every route of a story with the fields of the common ones |
| `story_overview` | | settings (calendar, months, eras), stats, article counts, volumes and chapters |
| `find` | | items by name, of every kind (also a character's other names) |
| `get_article` | | an item's wiki article, links shown by name |
| `update_article` | ✓ | change an article's summary, fields, sections or infobox; what isn't sent stays |
| `read_chapter` | | a chapter's text as Markdown, its version and words |
| `write_chapter` | ✓ | add Markdown after a chapter's text, or replace it (with the version it was read at) |
| `get` | | any `GET` route of the story |
| `send` | ✓ | any `POST`, `PUT` or `DELETE` route, with a JSON body or form fields |

Everything a tool does goes through the same checks, copy and log as a
plain request.

### Chapters as Markdown

Every line is a paragraph (blank lines only separate), `#`…`######` are
headings, `* * *`, `***` or `---` a scene break, `- ` and `1. ` lists,
`**bold**`, `*italic*`, `~~struck~~`, `[text](https://…)` a web link and
`[[Name]]`, `[[type:Name]]` or `[[Name|text]]` a link to a story item
(stored by id, so "Appears in" finds it). Reading shows what Markdown has
no word for as `**Speaker** (manner): text` for a dialogue, `![picture]`,
`[^note: …]`, `[table]`.

A replace refuses when the chapter holds what Markdown can't keep
(dialogues, pictures, notes, comments, styles, paragraph formatting) unless
`allow_loss` is true, and when the chapter changed since it was read. The
editor's own version history keeps the text before each save.

## Connecting a client

The address below is wherever your API is reached, with HTTPS from outside.

**Claude Code**

```bash
claude mcp add --transport http polaris https://api.example.com/mcp --header "Authorization: Bearer pol_…"
```

**Claude Desktop**: through `mcp-remote` in `claude_desktop_config.json`:

```json
{ "mcpServers": { "polaris": {
  "command": "npx",
  "args": ["mcp-remote", "https://api.example.com/mcp", "--header", "Authorization: Bearer ${POLARIS_KEY}"],
  "env": { "POLARIS_KEY": "pol_…" }
} } }
```

**Open WebUI**: add a tool server, as MCP (streamable HTTP, at `/mcp`) or
OpenAPI (at the API's address; it reads `/openapi.json`), with bearer
authentication and the key.

claude.ai's custom connectors sign in with OAuth, which the API doesn't
offer; use one of the clients above.
