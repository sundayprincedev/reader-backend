# meReader — backend

Go API for [meReader](https://github.com/sundayprincedev/reader-frontend): accounts, reading positions, and
book storage.

## What it stores

| Collection | Contents |
| --- | --- |
| `users` | Email and a bcrypt password hash |
| `books` | Fingerprint, title, author, format, position, history, time read |
| GridFS (`fs.*`) | The PDF / EPUB bytes |

A book is identified by a fingerprint the browser derives from the file (filename, byte size, and first
512 KB, hashed with SHA-256), so re-adding the same file finds the same book and keeps its progress.

## Run it

```bash
cp .env.example .env      # fill in MONGODB_URI and JWT_SECRET
go run .                  # http://localhost:8080
```

Generate a signing key with `openssl rand -hex 32`.

## Environment

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `MONGODB_URI` | yes | — | MongoDB connection string |
| `JWT_SECRET` | yes | — | Session signing key, 32+ characters |
| `MONGODB_DATABASE` | no | `mereader` | Database name |
| `PORT` | no | `8080` | Listen port (Railway sets this) |
| `ALLOWED_ORIGINS` | no | `*` | Comma-separated CORS origins |
| `MAX_UPLOAD_MB` | no | `80` | Largest book accepted |
| `STATIC_DIR` | no | — | Serve a built frontend from this directory |

Changing `JWT_SECRET` signs everyone out, so keep it stable once you are live.

## API

Everything except `/api/health` and the two auth routes needs `Authorization: Bearer <token>`.

| Method | Route | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | Liveness probe |
| `POST` | `/api/auth/register` | Create an account, returns a token |
| `POST` | `/api/auth/login` | Sign in, returns a token |
| `GET` | `/api/auth/me` | The signed-in account |
| `GET` | `/api/books` | Library plus aggregate stats |
| `POST` | `/api/books` | Register or update a book by fingerprint |
| `GET` | `/api/books/{key}` | One book with its saved position |
| `POST` | `/api/books/{key}/file` | Upload the book's bytes |
| `GET` | `/api/books/{key}/file` | Stream the book back |
| `PUT` | `/api/books/{key}/progress` | Save the current position |
| `POST` | `/api/books/{key}/reset` | Start the book over |
| `POST` | `/api/books/{key}/restore` | Jump back to a history checkpoint |
| `DELETE` | `/api/books/{key}` | Remove a book and its stored file |

Books belong to the account that created them; another account asking for one gets a 404 rather than a 403,
so the API never confirms that a fingerprint exists.

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
internal/auth               bcrypt hashing, JWT signing
internal/storage            Mongo connection, indexes, GridFS bucket
internal/models             documents and request/response shapes
internal/repository         data access for users, books, files
internal/api                handlers, middleware, routing
```

## Deploying on Railway

1. [railway.app](https://railway.app) → **New Project** → **Deploy from GitHub repo** → this repository.
   `railway.json` selects the Dockerfile and the `/api/health` healthcheck.
2. In **Variables**, set `MONGODB_URI`, `MONGODB_DATABASE`, `JWT_SECRET`, and `ALLOWED_ORIGINS`.
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
