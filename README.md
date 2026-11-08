# user-service

User accounts for the nexus-commerce platform: registration with email confirmation,
login (JWT), renaming, and email change with confirmation.

## Scope and a known compromise

This service deliberately merges two bounded contexts that would ideally be separate:

- **Identity** (generic subdomain) — credentials, login, confirmation tokens, email ownership.
- **Customer** (supporting subdomain) — the person's profile (name, and whatever the shop needs later).

Neither is the platform's core domain, so neither warrants its own bounded context;
a single `user` context is a deliberate trade-off, not a shortcut.

In a production system, Identity would be delegated to an off-the-shelf provider
(Keycloak, Auth0, Zitadel). Here it is built in-house on purpose: this is a learning
project, and modelling registration, confirmation and email change is part of the goal.

The model mostly keeps them apart: authentication and confirmation flows on one side,
`Rename` on the other. The seam is not perfectly clean: registration collects the name
together with credentials, and identity events carry
`FullName` for notifications. Extracting `identity-service` and `customer-service` later
therefore also means deciding:

- who creates the customer profile (likely `customer-service` reacting to `Registered`);
- where `notification-service` gets the name from (its own projection of customer
  events, or a display name kept by identity);
- whether registration stays a single form that writes to both contexts.

## Architecture

**Domain-Driven Design** for the model, **Clean Architecture** for the layering:
dependencies point inward only.

| Layer | Packages | Rule |
|---|---|---|
| Domain | `internal/user` | Aggregates, value objects, domain events. No infrastructure imports. |
| Application | `internal/usecase` | One type per use case; each declares the narrow repository interface it needs. |
| Adapters | `http`, `postgres`, `kafka`, `messaging`, `bcrypt`, `jwt`, `nacl` | Translate at the boundary: HTTP DTOs, Postgres rows and Kafka envelopes never reach the domain. |

Events are written to an **outbox** table in the same transaction as the aggregate,
then relayed to Kafka.

### Aggregates

| Aggregate | Responsibility |
|---|---|
| `PendingRegistration` | A registration awaiting email confirmation (token hash, 24h expiry). Acts as the factory for `User`: `Complete()` only succeeds once confirmed. A newer request for the same email supersedes an unconfirmed one (`Supersede()`): the registration keeps its ID and gets a fresh token and expiry. If the email is taken by the time it completes, it is rejected (`RegistrationRejected`). |
| `User` | A registered account. Owns authentication, renaming and email change. Holds at most one `PendingEmailChange`; requesting a change requires a `Reauthentication` proof obtained by re-entering the password. |

Confirmation tokens travel in events sealed with NaCl anonymous box using
notification-service's public key, so only that service can read them. Duplicate and
taken-address attempts are reported as events rather than API errors to avoid account
enumeration.

## API contracts

Served by `api` behind basic auth (`DOCS_USERNAME` / `DOCS_PASSWORD`):

- OpenAPI (HTTP): `/api/docs/openapi`
- AsyncAPI (events): `/api/docs/asyncapi`

## Entry points

One image, several binaries (`cmd/`):

| Command | Role |
|---|---|
| `api` | HTTP API on `/api` (default image `CMD`); runs DB migrations on start |
| `relay` | Polls the outbox and publishes to Kafka |
| `consumer` | Consumes `users.internal` and completes registrations |
| `purge` | Periodically deletes expired pending registrations and email changes |
| `decrypt-token` | Dev tool: decrypts a sealed token from an event (needs `NOTIFICATION_PRIVATE_KEY`) |

## Tech stack

Go 1.27.2 · PostgreSQL 15 · Kafka · Docker

## Requirements

- Docker

## Running

```sh
./dev/redeploy.sh           # api (port 80), purge, Postgres (port 5433)
./dev/redeploy.sh --kafka   # plus Kafka (port 9092), outbox-relay, registration-consumer
```

The `.env` file contains working development values. Optional settings and their
defaults are in [internal/config/config.go](internal/config/config.go).

Without Kafka, events accumulate in the outbox and confirmed registrations are not
completed until the `kafka` profile runs. The script recreates the containers and
wipes the database volume.

## Tests

```sh
./dev/test.sh   # all tests, including PostgreSQL integration tests
```

Tests run inside the dev container against the compose Postgres, using credentials
from `.env`. Arguments are passed to `go test`. Each integration test creates a unique schema, applies the migrations,
and drops only that schema on cleanup. Kafka is not required.
