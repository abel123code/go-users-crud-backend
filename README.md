# Go Backend with Neon Postgres

A simple REST API backend built with Go and Neon Postgres database. Purpose of building this was to allow myself to get use to Go's syntax when building backends

## Setup

1. Install dependencies:

   ```bash
   go mod download
   ```

2. Create a `.env` file in the project root:

   ```
   DATABASE_URL=postgres://user:password@host:port/dbname
   TEST_DATABASE_URL=postgres://user:password@host:port/dbname
   ```

3. Run the server:

   ```bash
   go run .
   ```

   Or use Air for live reloading:

   ```bash
   air
   ```

Server runs on `http://localhost:8080`

## Routes

- `GET /health` - Health check endpoint, verifies database connection
- `GET /users` - List all users
- `POST /users` - Create a new user (requires `firstName` and `lastName` in JSON body)
- `GET /users/{id}` - Get a user by ID (returns 404 if not found)
- `PATCH /users/{id}` - Partially update a user by ID
- `DELETE /users/{id}` - Delete a user by ID (returns 404 if not found)

## Testing

Run tests:

```bash
go test -v
```

Tests require `DATABASE_URL` or `TEST_DATABASE_URL` environment variable. Tests include:

- `TestHealth` - Verifies health endpoint
- `TestCreateUser` - Tests user creation
- `TestGetUsersByID` - Tests retrieving a user by ID
- `TestGetUsersByIDNotFound` - Tests 404 handling for non-existent users

## System Design Decisions

1. Used Context library for each handler. Passed Context to sql query functions so that we can get more accurate accurate error message (Timeout or Request cancelled)

2. Implemented middleware (logging, requestID, panic middleware)

panicMiddleware.ServeHTTP
↓
loggingMiddleware.ServeHTTP
↓
requestIDMiddleware.ServeHTTP
↓
handler.ServeHTTP
↑
requestIDMiddleware returns
↑
loggingMiddleware returns
↑
panicMiddleware returns

- **Request ID**

  - Ensures every request has a unique `X-Request-ID`
  - Stored in `context.Context`
  - Propagated to logs and responses for traceability

- **Logging**

  - Logs request start/end
  - Includes method, path, status code, duration
  - Correlates logs using request ID

- **Panic recovery**
  - Catches unexpected panics from handlers or middleware
  - Logs panic + stack trace with request ID
  - Returns a clean `500 Internal Server Error`
  - Prevents a single request from crashing the server
