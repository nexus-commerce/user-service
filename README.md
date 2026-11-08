# user-service

Run checks with Go 1.27.2 or newer:

```powershell
go test ./...
go vet ./...
```

PostgreSQL integration tests are skipped unless `TEST_POSTGRES_DSN` is set.
Use a test database with permission to create schemas. Each test creates a unique
schema, applies the existing migrations, and drops only that schema on cleanup.
Kafka is not required.

```powershell
$env:TEST_POSTGRES_DSN = 'postgres://test_user:test_password@localhost:5433/test_database?sslmode=disable'
go test ./internal/postgres -count=1 -v
```
