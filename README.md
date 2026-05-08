# go-reliable-webhook

A production-minded reliable webhook delivery system built with Go, Fiber, and PostgreSQL.

This repository currently contains the initial API foundation only. Subscriber registration, event creation, delivery processing, retries, replay, and workers are intentionally not implemented yet.

## Local Setup

Start PostgreSQL:

```sh
docker compose up -d postgres
```

Set required environment variables:

```sh
export APP_ENV=development
export PORT=8080
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/reliable_webhook?sslmode=disable'
export HTTP_CLIENT_TIMEOUT_SECONDS=10
```

Run the API:

```sh
make run
```

Check health:

```sh
curl http://localhost:8080/api/v1/health
```

Run tests:

```sh
make test
```

Run migrations when migration files exist:

```sh
make migrate-up
make migrate-down
```
