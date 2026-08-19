# meReader — backend

Go API for [meReader](https://github.com/sundayprincedev/reader-frontend): accounts, reading positions, and
book storage.

## What it stores

| Collection | Contents |
| --- | --- |
| `books` | Fingerprint, title, author, format, position, history, time read |
| GridFS (`fs.*`) | The PDF / EPUB bytes |

A book is identified by a fingerprint the browser derives from the file (filename, byte size, and first
512 KB, hashed with SHA-256), so re-adding the same file finds the same book and keeps its progress.

## Run it

```bash
cp .env.example .env      # fill in MONGODB_URI
go run .                  # http://localhost:8080
```

## The PIN

There are no accounts — one shelf, and every request sees it. Instead of a login, the API is gated by a
shared PIN sent as an `X-Reader-Pin` header. Set it with `ACCESS_PIN`; leave it unset and every request is
allowed through, which the server warns about at startup.

`/api/health` stays open so Railway's healthcheck keeps working. Everything else needs the PIN.

Wrong PINs are rate limited: five failures from an address trigger a 15 minute lockout, and the correct PIN
is refused during it. A second cap covers the whole service, so rotating addresses does not help. That
turns a short numeric PIN from something a script cracks in seconds into something that would take years.

It is still a shared secret in a header, not real authentication. It keeps out scanners and passers-by; it
would not stop someone determined who knows the URL.

## Environment

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `MONGODB_URI` | yes | — | MongoDB connection string |
| `ACCESS_PIN` | no | — | Gates the API; unset means open |
| `MONGODB_DATABASE` | no | `mereader` | Database name |
| `PORT` | no | `8080` | Listen port (Railway sets this) |
| `ALLOWED_ORIGINS` | no | `*` | Comma-separated CORS origins |
| `MAX_UPLOAD_MB` | no | `80` | Largest book accepted |
| `STATIC_DIR` | no | — | Serve a built frontend from this directory |

## API

| Method | Route | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Liveness probe, not gated |
| `POST` | `/api/unlock` | Check a PIN; 204 when correct |
| `GET` | `/api/books` | Library plus aggregate stats |
| `POST` | `/api/books` | Register or update a book by fingerprint |
| `GET` | `/api/books/{key}` | One book with its saved position |
| `POST` | `/api/books/{key}/file` | Upload the book's bytes |
| `GET` | `/api/books/{key}/file` | Stream the book back |
| `PUT` | `/api/books/{key}/progress` | Save the current position |
| `POST` | `/api/books/{key}/reset` | Start the book over |
| `POST` | `/api/books/{key}/restore` | Jump back to a history checkpoint |
| `DELETE` | `/api/books/{key}` | Remove a book and its stored file |

Uploads are checked against the format's magic number, so a file that is not really a PDF or EPUB is
rejected rather than stored. A book's bytes can only be attached once; re-uploading returns the existing
book instead of overwriting it. The stored size is measured server-side, not taken from the client.

### Removing a book

A book can only be removed once you have finished it, or if you have never started it. Anything in
progress returns `409 Conflict`, so you cannot abandon a book halfway and clear it away.

Starting a book over does not make it removable again: the check looks at the reading history, which a
reset preserves. Every book response carries `started` and `removable` so clients can reflect the rule
without reimplementing it.

## Layout

```
main.go                     startup, graceful shutdown
internal/config             environment loading
internal/storage            Mongo connection, indexes, GridFS bucket
internal/models             documents and request/response shapes
internal/repository         data access for books and files
internal/api                handlers, middleware, PIN gate, routing
```

## Deploying on Railway

1. [railway.app](https://railway.app) → **New Project** → **Deploy from GitHub repo** → this repository.
   `railway.json` selects the Dockerfile and the `/api/health` healthcheck.
2. In **Variables**, set `MONGODB_URI`, `MONGODB_DATABASE`, `ACCESS_PIN`, and `ALLOWED_ORIGINS`.
   Do **not** set `PORT` — Railway injects it and the server already reads it.
3. In Atlas → **Network Access**, allow `0.0.0.0/0`. Railway has no fixed egress IPs on the standard plan,
   and deploys time out until this is done.
4. **Settings** → **Networking** → **Generate Domain**, then check it:

   ```bash
   curl https://<your-service>.up.railway.app/api/health
   # {"status":"ok"}
   ```

5. Point the frontend's `VITE_API_URL` at that domain, then set `ALLOWED_ORIGINS` here to the frontend's
   domain and redeploy.

Atlas's free tier gives 512 MB, which covers roughly 20–50 books. Deleting a book frees its space.
