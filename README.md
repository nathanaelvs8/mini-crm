# Mini CRM

A small CRM I built to practice full-stack work. You keep a list of customers, log calls, emails and meetings against each one, and move them through a sales pipeline (lead, contacted, negotiation, won, lost).

Go REST API, PostgreSQL, and a React + TypeScript frontend (Vite).

## Features

- Sign up / sign in, each user only sees their own data
- Add, edit, search and delete customers
- Log calls, emails, meetings and notes per customer
- Pipeline stages with per-stage counts and a stage filter
- Board view with one column per stage

## Structure

```
backend/
  cmd/server/        production entrypoint (PostgreSQL)
  cmd/devserver/     same API with in-memory data, no database needed
  internal/model/    User, Customer, Interaction
  internal/store/    Store interface + in-memory implementation
  internal/store/pg/ PostgreSQL implementation (pgx) and schema.sql
  internal/auth/     password hashing and tokens
  internal/httpapi/  routes, handlers, middleware, API tests
frontend/
  src/api.ts         all backend calls
  src/auth.tsx       login state
  src/pages/         Login, Customers, Customer detail, Pipeline
```

## Running it

You need Go 1.24+, Node 20+, and either Docker or your own PostgreSQL 14+.

Start PostgreSQL (skip if you already have one):

```
docker compose up -d
```

Start the API. Tables are created on startup.

```
cd backend
go mod tidy
export DATABASE_URL="postgres://crm:crm@localhost:5432/crm?sslmode=disable"
export JWT_SECRET="some-long-random-string"
go run ./cmd/server
```

Start the frontend in a second terminal:

```
cd frontend
npm install
npm run dev
```

Then open http://localhost:5173 and register an account.

If you just want to try it without a database, `go run ./cmd/devserver` runs the same API with in-memory data (gone on restart).

## Tests

```
cd backend && go test ./internal/...
cd frontend && npm test
```

The API tests run against the in-memory store.

## API

Everything except register and login needs `Authorization: Bearer <token>`.

| Method | Route | Notes |
| --- | --- | --- |
| POST | `/api/auth/register` | `{name, email, password}` returns `{token, user}` |
| POST | `/api/auth/login` | `{email, password}` returns `{token, user}` |
| GET | `/api/me` | current user |
| GET | `/api/stats` | customers per stage |
| GET | `/api/customers?q=&stage=` | search by name, email or company; filter by stage |
| POST | `/api/customers` | |
| GET / PUT / DELETE | `/api/customers/{id}` | |
| PATCH | `/api/customers/{id}/stage` | `{stage}` |
| GET / POST | `/api/customers/{id}/interactions` | POST takes `{kind, content}` |
| DELETE | `/api/interactions/{id}` | |

Errors come back as `{"error": "message"}`.

## Notes on a few decisions

- Handlers depend on a `Store` interface, so the same code runs on PostgreSQL normally and on plain maps in tests.
- Every query filters by `owner_id`. Asking for someone else's row gives a 404 instead of a 403 so IDs can't be probed. There's a test for it.
- Login returns the same error for an unknown email and a wrong password.
- Passwords use PBKDF2-SHA256 (600k iterations, random salt) and tokens are HS256 JWTs, both hand-written with the standard library to avoid extra dependencies. For a real product I'd use bcrypt/argon2 and a maintained JWT library.
- The token lives in `localStorage`. Simple, but an `httpOnly` cookie would hold up better against XSS.

## Deploying

- Database: any hosted PostgreSQL (Neon, Supabase, Railway)
- API: anything that runs a Go binary or Docker (Render, Railway, Fly.io). Set `DATABASE_URL`, `JWT_SECRET` and `ALLOWED_ORIGIN` (the frontend URL)
- Frontend: Vercel or Netlify. Set `VITE_API_URL` to the API URL and build with `npm run build`

## To do

- Tasks and reminders per customer
- Drag and drop on the board
- Pagination
- Proper migrations (golang-migrate) instead of one schema file
