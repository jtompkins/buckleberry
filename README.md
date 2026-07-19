# buckleberry

A simple OPDS server that exposes a [Wallabag](https://www.wallabag.org/) instance to OPDS clients (e-readers, KOReader, etc.). Browse your unread articles and download them as ePUB.

## Running

```sh
make run      # dev
make build    # -> out/buckleberry
make test
```

On first launch, open the server in a browser and complete onboarding to set your admin login and Wallabag credentials.

## Configuration

Set via environment variables or a `.env` file:

| Variable        | Default                  | Description                                        |
| --------------- | ------------------------ | -------------------------------------------------- |
| `PORT`          | `8080`                   | Port to listen on.                                 |
| `BASE_URL`      | `http://localhost:$PORT` | Public URL, used to build OPDS feed links.         |
| `DB_PATH`       | `./buckleberry.db`       | SQLite database path.                              |
| `COOKIE_SECURE` | `true` if `BASE_URL` is HTTPS | Set the `Secure` flag on the session cookie.  |

## Endpoints

- `/onboarding`, `/`, `/settings` — web UI (session auth).
- `/opds`, `/opds/unread`, `/opds/download/:id` — OPDS feeds (HTTP Basic auth).

Point your OPDS reader at `<BASE_URL>/opds` and log in with your admin credentials.

## Deployment

The `Dockerfile` builds a static, CGO-free binary onto `distroless/static` (runs as non-root, uid 65532).

For [Coolify](https://coolify.io/):

1. Point a new resource at this repo; it builds the `Dockerfile` automatically.
2. Add a **persistent volume mounted at `/data`** — the SQLite DB lives there and would otherwise reset on redeploy. The container runs non-root and can't `chown` the mount, so if you get a permission error on the DB, make the host path owned by `65532:65532`.
3. Set env vars:
   - `BASE_URL` → your public URL (e.g. `https://buckleberry.example.com`). Required — OPDS links are built from it.
   - `COOKIE_SECURE` is auto-`true` when `BASE_URL` is HTTPS, so behind Coolify's TLS you're covered.

## Development

Views are [templ](https://templ.guide/) components (`*.templ` → generated `*_templ.go`).

- **Generating:** `make generate` (or `make watch` for live reload). This runs `go tool templ generate`, which uses the version pinned in `go.mod` — no manual install needed for builds. The generated `*_templ.go` files are committed.
- **Editor / LSP:** your editor's templ plugin needs the `templ` binary on your `PATH`. Install it globally, matching the pinned version to avoid drift:

  ```sh
  go install github.com/a-h/templ/cmd/templ@v0.3.1020
  ```

- If the `tool` directive ever goes missing from `go.mod`, re-add it with `go get -tool github.com/a-h/templ/cmd/templ`.
