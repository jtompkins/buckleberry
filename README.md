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
