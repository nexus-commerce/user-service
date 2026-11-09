package postgres

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/nexus-commerce/user-service/internal/messaging"
	"github.com/nexus-commerce/user-service/internal/usecase"
	"github.com/nexus-commerce/user-service/internal/user"
	"github.com/nexus-commerce/user-service/internal/userevent"
)

// testDB gives each test its own search_path, including its own migration history.
func testDB(t *testing.T) *Database {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	logger := slog.New(slog.DiscardHandler)
	admin, err := New(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Close(); err != nil {
			t.Errorf("close admin db: %v", err)
		}
	})
	schema := "postgres_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.pool.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatalf("CREATE SCHEMA %s error: %v", quoted, err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.pool.ExecContext(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("clean up test schema: %v", err)
		}
	})
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig() error: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	db := &Database{pool: stdlib.OpenDB(*config), logger: logger}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test db: %v", err)
		}
	})
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}
	return db
}

type testSealer struct{}

func (testSealer) Seal(p []byte) ([]byte, error) { return p, nil }

func (testSealer) KeyID() string { return "test" }

type testRepos struct {
	db            *Database
	users         *Users
	registrations *Registrations
}

func newTestRepos(t *testing.T) testRepos {
	t.Helper()
	db := testDB(t)
	publisher, err := userevent.NewPublisher(testSealer{})
	if err != nil {
		t.Fatalf("userevent.NewPublisher() error: %v", err)
	}
	return testRepos{db: db, users: NewUsers(db, publisher), registrations: NewRegistrations(db, publisher)}
}

func (r testRepos) confirm() *usecase.ConfirmRegistration {
	return usecase.NewConfirmRegistration(r.registrations)
}

func (r testRepos) complete() *usecase.CompleteRegistration {
	return usecase.NewCompleteRegistration(r.registrations, r.users)
}

func (r testRepos) seedRegistration(t *testing.T, email string) (*user.PendingRegistration, user.ConfirmationToken) {
	t.Helper()
	token := user.NewConfirmationToken()
	expiresAt := time.Now().UTC().Add(time.Hour)
	pending := user.RestorePendingRegistration(user.PendingRegistrationSnapshot{
		UserID:       uuid.New(),
		Email:        user.RestoreEmail(email),
		FullName:     testName("Jane", "Doe"),
		PasswordHash: user.RestorePasswordHash("test-hash"),
		TokenHash:    token.Hash(),
		ExpiresAt:    expiresAt,
	})
	if err := r.registrations.Save(t.Context(), pending, supersedeWith(pending)); err != nil {
		t.Fatalf("Registrations.Save() error: %v", err)
	}
	return pending, token
}

func (r testRepos) confirmPending(t *testing.T, token user.ConfirmationToken) {
	t.Helper()
	_, err := r.registrations.UpdateByToken(t.Context(), token.Hash(), func(p *user.PendingRegistration) error {
		return p.Confirm(token)
	})
	if err != nil {
		t.Fatalf("Registrations.UpdateByToken() error: %v", err)
	}
}

type registrationFields struct {
	FirstName    string
	LastName     string
	PasswordHash string
	TokenHash    string
}

func registrationOf(p *user.PendingRegistration) registrationFields {
	return registrationFields{
		FirstName:    p.FullName().FirstName().String(),
		LastName:     p.FullName().LastName().String(),
		PasswordHash: p.PasswordHash().Encoded(),
		TokenHash:    p.TokenHash().Encoded(),
	}
}

func testName(first, last string) user.FullName {
	return user.NewFullName(user.RestoreName(first), user.RestoreName(last))
}

func supersedeWith(p *user.PendingRegistration) user.PendingRegistrationUpdate {
	return func(existing *user.PendingRegistration) error {
		return existing.Supersede(p)
	}
}

func (r testRepos) seedUser(t *testing.T, email string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := r.db.pool.ExecContext(t.Context(), `INSERT INTO users (id, first_name, last_name, email, password_hash)
		VALUES ($1, 'Other', 'User', $2, 'hash')`, id.String(), email)
	if err != nil {
		t.Fatalf("INSERT INTO users error: %v", err)
	}
	return id
}

func (r testRepos) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := r.db.pool.QueryRowContext(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q error: %v", query, err)
	}
	return n
}

type userState struct {
	Users   int
	Pending int
	Events  map[string]int
}

func (r testRepos) stateOf(t *testing.T, id uuid.UUID) userState {
	t.Helper()
	s := userState{
		Users:   r.count(t, `SELECT count(*) FROM users WHERE id = $1`, id.String()),
		Pending: r.count(t, `SELECT count(*) FROM pending_registrations WHERE user_id = $1`, id.String()),
	}
	const query = `SELECT headers->>$2, count(*) FROM outbox WHERE key = convert_to($1, 'UTF8') GROUP BY 1`
	rows, err := r.db.pool.QueryContext(t.Context(), query, id.String(), messaging.HeaderEventType)
	if err != nil {
		t.Fatalf("query %q error: %v", query, err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		var n int
		if err := rows.Scan(&typ, &n); err != nil {
			t.Fatalf("scan outbox event count: %v", err)
		}
		if s.Events == nil {
			s.Events = make(map[string]int)
		}
		s.Events[typ] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate outbox event counts: %v", err)
	}
	return s
}

func TestConfirmThenCompleteRegistration(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")

	if err := r.confirm().Execute(t.Context(), token.Plaintext()); err != nil {
		t.Fatalf("ConfirmRegistration.Execute() error: %v", err)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 1, Events: map[string]int{userevent.TypeRegistrationConfirmed: 1}}, r.stateOf(t, pending.UserID())); diff != "" {
		t.Errorf("after ConfirmRegistration.Execute(): state of %v mismatch (-want +got):\n%s", pending.UserID(), diff)
	}

	for i := range 2 {
		if err := r.complete().Execute(t.Context(), pending.UserID()); err != nil {
			t.Fatalf("CompleteRegistration.Execute() #%d error: %v", i+1, err)
		}
	}
	if diff := cmp.Diff(userState{Users: 1, Pending: 0, Events: map[string]int{
		userevent.TypeRegistrationConfirmed: 1,
		"user.registered":                   1,
	}}, r.stateOf(t, pending.UserID())); diff != "" {
		t.Errorf("after CompleteRegistration.Execute(): state of %v mismatch (-want +got):\n%s", pending.UserID(), diff)
	}
	if err := r.confirm().Execute(t.Context(), token.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("repeated ConfirmRegistration.Execute() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestCompleteRegistrationRejectsTakenEmail(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	r.seedUser(t, "jane@example.com")

	if err := r.confirm().Execute(t.Context(), token.Plaintext()); err != nil {
		t.Fatalf("ConfirmRegistration.Execute() error: %v", err)
	}
	if err := r.complete().Execute(t.Context(), pending.UserID()); !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("CompleteRegistration.Execute() error = %v, want %v", err, user.ErrEmailTaken)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 0, Events: map[string]int{
		userevent.TypeRegistrationConfirmed: 1,
		"user.registration_rejected":        1,
	}}, r.stateOf(t, pending.UserID())); diff != "" {
		t.Errorf("after CompleteRegistration.Execute(): state of %v mismatch (-want +got):\n%s", pending.UserID(), diff)
	}
}

func TestDuplicateRejectionPublishesOnce(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	if err := r.confirm().Execute(t.Context(), token.Plaintext()); err != nil {
		t.Fatalf("ConfirmRegistration.Execute() error: %v", err)
	}
	var rejected []*user.PendingRegistration
	for range 2 {
		p, err := r.registrations.ByUserID(t.Context(), pending.UserID())
		if err != nil {
			t.Fatalf("Registrations.ByUserID() error: %v", err)
		}
		if err = p.Reject(); err != nil {
			t.Fatalf("Reject() error: %v", err)
		}
		rejected = append(rejected, p)
	}

	for i, p := range rejected {
		if err := r.registrations.Remove(t.Context(), p); err != nil {
			t.Errorf("Registrations.Remove() #%d error: %v", i+1, err)
		}
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 0, Events: map[string]int{
		userevent.TypeRegistrationConfirmed: 1,
		"user.registration_rejected":        1,
	}}, r.stateOf(t, pending.UserID())); diff != "" {
		t.Errorf("after Registrations.Remove(): state of %v mismatch (-want +got):\n%s", pending.UserID(), diff)
	}
}

func TestUpdateByTokenRollsBackOnFailure(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	want := errors.New("update failed")

	_, err := r.registrations.UpdateByToken(t.Context(), token.Hash(), func(p *user.PendingRegistration) error {
		if err := p.Confirm(token); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Errorf("Registrations.UpdateByToken() error = %v, want %v", err, want)
	}
	p, err := r.registrations.ByUserID(t.Context(), pending.UserID())
	if err != nil {
		t.Fatalf("Registrations.ByUserID() error: %v", err)
	}
	if p.Confirmed() {
		t.Error("Registrations.UpdateByToken() kept the confirmation after a failed update, want it rolled back")
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 1}, r.stateOf(t, pending.UserID())); diff != "" {
		t.Errorf("after Registrations.UpdateByToken(): state of %v mismatch (-want +got):\n%s", pending.UserID(), diff)
	}
}

func TestConcurrentConfirmation(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	confirm := r.confirm()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			results <- confirm.Execute(ctx, token.Plaintext())
		}()
	}
	close(start)
	var successes, invalid int
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, user.ErrInvalidConfirmationToken):
			invalid++
		default:
			t.Errorf("ConfirmRegistration.Execute() error = %v, want nil or %v", err, user.ErrInvalidConfirmationToken)
		}
	}
	if successes != 1 || invalid != 1 {
		t.Errorf("concurrent ConfirmRegistration.Execute() successes, invalid tokens = %d, %d; want 1, 1", successes, invalid)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 1, Events: map[string]int{userevent.TypeRegistrationConfirmed: 1}}, r.stateOf(t, pending.UserID())); diff != "" {
		t.Errorf("after concurrent ConfirmRegistration.Execute(): state of %v mismatch (-want +got):\n%s", pending.UserID(), diff)
	}
}

func TestSaveKeepsConfirmedRegistration(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	r.confirmPending(t, token)

	again := newTestRegistration(t, "jane@example.com", testName("Jane", "Doe"))
	if err := r.registrations.Save(t.Context(), again, supersedeWith(again)); !errors.Is(err, user.ErrRegistrationAlreadyConfirmed) {
		t.Errorf("Registrations.Save() error = %v, want %v", err, user.ErrRegistrationAlreadyConfirmed)
	}
	p, err := r.registrations.ByUserID(t.Context(), pending.UserID())
	if err != nil {
		t.Fatalf("Registrations.ByUserID() error: %v", err)
	}
	if !p.Confirmed() {
		t.Error("Registrations.Save() reset the confirmation of a confirmed registration")
	}
	if got, want := p.TokenHash().Encoded(), token.Hash().Encoded(); got != want {
		t.Errorf("Registrations.Save() changed the token hash of a confirmed registration to %q, want %q", got, want)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 1, Events: map[string]int{userevent.TypeRegistrationConfirmed: 1}}, r.stateOf(t, pending.UserID())); diff != "" {
		t.Errorf("after Registrations.Save(): state of %v mismatch (-want +got):\n%s", pending.UserID(), diff)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 0}, r.stateOf(t, again.UserID())); diff != "" {
		t.Errorf("after Registrations.Save(): state of %v mismatch (-want +got):\n%s", again.UserID(), diff)
	}
}

func newTestRegistration(t *testing.T, email string, name user.FullName) *user.PendingRegistration {
	t.Helper()
	p, err := user.RequestRegistration(user.RestoreEmail(email), name, mustTestPassword(t), testHasher{})
	if err != nil {
		t.Fatalf("RequestRegistration() error: %v", err)
	}
	return p
}

func TestSaveSupersedesUnconfirmedRegistration(t *testing.T) {
	r := newTestRepos(t)
	first, oldToken := r.seedRegistration(t, "jane@example.com")

	again := newTestRegistration(t, "jane@example.com", testName("Janet", "Roe"))
	var superseded *user.PendingRegistration
	err := r.registrations.Save(t.Context(), again, func(existing *user.PendingRegistration) error {
		superseded = existing
		return supersedeWith(again)(existing)
	})
	if err != nil {
		t.Fatalf("Registrations.Save() error: %v", err)
	}
	if superseded == nil {
		t.Fatal("Registrations.Save() did not call the update for the existing registration")
	}
	p, err := r.registrations.ByUserID(t.Context(), first.UserID())
	if err != nil {
		t.Fatalf("Registrations.ByUserID(first) error: %v", err)
	}
	want := registrationFields{
		FirstName:    "Janet",
		LastName:     "Roe",
		PasswordHash: again.PasswordHash().Encoded(),
		TokenHash:    superseded.TokenHash().Encoded(),
	}
	if diff := cmp.Diff(want, registrationOf(p)); diff != "" {
		t.Errorf("Registrations.Save() stored registration mismatch (-want +got):\n%s", diff)
	}
	if !p.ExpiresAt().After(first.ExpiresAt()) {
		t.Errorf("Registrations.Save() ExpiresAt() = %v, want after %v", p.ExpiresAt(), first.ExpiresAt())
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 1, Events: map[string]int{"user.registration_requested": 1}}, r.stateOf(t, first.UserID())); diff != "" {
		t.Errorf("after Registrations.Save(): state of %v mismatch (-want +got):\n%s", first.UserID(), diff)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 0}, r.stateOf(t, again.UserID())); diff != "" {
		t.Errorf("after Registrations.Save(): state of %v mismatch (-want +got):\n%s", again.UserID(), diff)
	}
	if err := r.confirm().Execute(t.Context(), oldToken.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("ConfirmRegistration.Execute(superseded token) error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestConcurrentSaveOfNewEmail(t *testing.T) {
	r := newTestRepos(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	regs := []*user.PendingRegistration{
		newTestRegistration(t, "jane@example.com", testName("Jane", "Doe")),
		newTestRegistration(t, "jane@example.com", testName("Jane", "Doe")),
	}
	start := make(chan struct{})
	results := make(chan error, len(regs))
	for _, p := range regs {
		go func() {
			<-start
			results <- r.registrations.Save(ctx, p, supersedeWith(p))
		}()
	}
	close(start)
	for range regs {
		if err := <-results; err != nil {
			t.Errorf("concurrent Registrations.Save() error: %v", err)
		}
	}
	if got := r.count(t, `SELECT count(*) FROM pending_registrations WHERE email = $1`, "jane@example.com"); got != 1 {
		t.Errorf("pending registrations for jane@example.com = %d, want 1", got)
	}
}

func TestUsersAddIsIdempotent(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	if err := pending.Confirm(token); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	u, err := pending.Complete()
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	for i := range 2 {
		if err := r.users.Add(t.Context(), u); err != nil {
			t.Fatalf("Users.Add() #%d error: %v", i+1, err)
		}
	}
	if diff := cmp.Diff(userState{Users: 1, Pending: 1, Events: map[string]int{"user.registered": 1}}, r.stateOf(t, u.ID())); diff != "" {
		t.Errorf("after Users.Add(): state of %v mismatch (-want +got):\n%s", u.ID(), diff)
	}
}

func TestUsersAddEmailTaken(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	r.seedUser(t, "jane@example.com")
	if err := pending.Confirm(token); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	u, err := pending.Complete()
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	if err := r.users.Add(t.Context(), u); !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("Users.Add() error = %v, want %v", err, user.ErrEmailTaken)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 1}, r.stateOf(t, u.ID())); diff != "" {
		t.Errorf("after Users.Add(): state of %v mismatch (-want +got):\n%s", u.ID(), diff)
	}
}

func TestDeleteAbandonedKeepsConfirmedRegistrations(t *testing.T) {
	r := newTestRepos(t)
	unconfirmed, _ := r.seedRegistration(t, "old@example.com")
	confirmed, token := r.seedRegistration(t, "jane@example.com")
	r.confirmPending(t, token)

	n, err := NewRetention(r.db).DeleteAbandonedRegistrations(t.Context(), time.Now().Add(2*time.Hour))
	if err != nil {
		t.Errorf("DeleteAbandonedRegistrations() error: %v", err)
	}
	if n != 1 {
		t.Errorf("DeleteAbandonedRegistrations() = %d, want 1", n)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 0}, r.stateOf(t, unconfirmed.UserID()), cmpopts.IgnoreFields(userState{}, "Events")); diff != "" {
		t.Errorf("after DeleteAbandonedRegistrations(): state of %v mismatch (-want +got):\n%s", unconfirmed.UserID(), diff)
	}
	if diff := cmp.Diff(userState{Users: 0, Pending: 1}, r.stateOf(t, confirmed.UserID()), cmpopts.IgnoreFields(userState{}, "Events")); diff != "" {
		t.Errorf("after DeleteAbandonedRegistrations(): state of %v mismatch (-want +got):\n%s", confirmed.UserID(), diff)
	}
}

func TestOutboxCarriesEventType(t *testing.T) {
	r := newTestRepos(t)
	pending, token := r.seedRegistration(t, "jane@example.com")
	r.confirmPending(t, token)

	var got []messaging.Message
	_, err := NewOutbox(r.db).ProcessBatch(t.Context(), 10, func(_ context.Context, msgs []messaging.Message) error {
		got = msgs
		return nil
	})
	if err != nil {
		t.Fatalf("Outbox.ProcessBatch() error: %v", err)
	}
	type routing struct{ Type, Key string }
	var gotRouting []routing
	for _, m := range got {
		gotRouting = append(gotRouting, routing{m.Type, m.Key})
	}
	want := []routing{{userevent.TypeRegistrationConfirmed, pending.UserID().String()}}
	if diff := cmp.Diff(want, gotRouting); diff != "" {
		t.Errorf("Outbox.ProcessBatch() messages mismatch (-want +got):\n%s", diff)
	}
}

type testHasher struct{}

func (testHasher) Hash(p user.Password) (user.PasswordHash, error) {
	return user.NewPasswordHash("hashed:" + p.Plaintext())
}

func (testHasher) Verify(h user.PasswordHash, p user.Password) (bool, error) {
	return h.Encoded() == "hashed:"+p.Plaintext(), nil
}

func mustTestPassword(t *testing.T) user.Password {
	t.Helper()
	p, err := user.NewPassword("s3cretpass")
	if err != nil {
		t.Fatalf("NewPassword() error: %v", err)
	}
	return p
}

func (r testRepos) seedPendingEmailChange(t *testing.T, id uuid.UUID, email string, token user.ConfirmationToken) {
	t.Helper()
	_, err := r.db.pool.ExecContext(t.Context(), `INSERT INTO pending_email_changes (user_id, email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`, id.String(), email, token.Hash().Encoded(), time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("INSERT INTO pending_email_changes error: %v", err)
	}
}

func (r testRepos) runBlockedBy(t *testing.T, change func(context.Context, *sql.Tx) error, op func(context.Context) error) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	tx, err := r.db.pool.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	var pid int
	if err := tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("SELECT pg_backend_pid() error: %v", err)
	}
	if err := change(ctx, tx); err != nil {
		t.Fatalf("blocking change error: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- op(ctx) }()
	for blocked := false; !blocked; {
		query := `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`
		if err := r.db.pool.QueryRowContext(ctx, query, pid).Scan(&blocked); err != nil {
			t.Fatalf("query pg_blocking_pids error: %v", err)
		}
		select {
		case err := <-done:
			t.Fatalf("operation finished before blocking on the lock: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}
	return <-done
}

func consumeEmailChange(id uuid.UUID, newEmail string) func(context.Context, *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET email = $1 WHERE id = $2`, newEmail, id.String()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM pending_email_changes WHERE user_id = $1`, id.String())
		return err
	}
}

func (r testRepos) rename(id uuid.UUID) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := r.users.UpdateByID(ctx, id, func(u *user.User) error {
			u.Rename(testName("Renamed", "User"))
			return nil
		})
		return err
	}
}

func (r testRepos) emailChangedEvents(t *testing.T, id uuid.UUID) int {
	t.Helper()
	query := `SELECT count(*) FROM outbox WHERE key = convert_to($1, 'UTF8') AND headers->>$2 = 'user.email_changed'`
	return r.count(t, query, id.String(), messaging.HeaderEventType)
}

func TestRenameWaitingOnConfirmedEmailChangeDoesNotRestoreIt(t *testing.T) {
	r := newTestRepos(t)
	id := r.seedUser(t, "old@example.com")
	r.seedPendingEmailChange(t, id, "new@example.com", user.NewConfirmationToken())

	if err := r.runBlockedBy(t, consumeEmailChange(id, "new@example.com"), r.rename(id)); err != nil {
		t.Fatalf("Users.UpdateByID(rename) error: %v", err)
	}

	if got := r.count(t, `SELECT count(*) FROM pending_email_changes WHERE user_id = $1`, id.String()); got != 0 {
		t.Errorf("pending email changes for %v = %d, want 0", id, got)
	}
	if got := r.count(t, `SELECT count(*) FROM users WHERE id = $1 AND first_name = 'Renamed' AND email = 'new@example.com'`, id.String()); got != 1 {
		t.Errorf("users with the new name and the new email = %d, want 1", got)
	}
}

func TestRenameWaitingOnRequestedEmailChangeKeepsIt(t *testing.T) {
	r := newTestRepos(t)
	id := r.seedUser(t, "old@example.com")
	token := user.NewConfirmationToken()

	request := func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET first_name = first_name WHERE id = $1`, id.String()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO pending_email_changes (user_id, email, token_hash, expires_at)
			VALUES ($1, 'new@example.com', $2, $3)`, id.String(), token.Hash().Encoded(), time.Now().UTC().Add(time.Hour))
		return err
	}
	if err := r.runBlockedBy(t, request, r.rename(id)); err != nil {
		t.Fatalf("Users.UpdateByID(rename) error: %v", err)
	}

	if got := r.count(t, `SELECT count(*) FROM pending_email_changes WHERE user_id = $1 AND token_hash = $2`, id.String(), token.Hash().Encoded()); got != 1 {
		t.Errorf("pending email changes with the mailed token = %d, want 1", got)
	}
}

func TestConfirmWaitingOnConfirmedEmailChangeRejectsUsedToken(t *testing.T) {
	r := newTestRepos(t)
	id := r.seedUser(t, "old@example.com")
	token := user.NewConfirmationToken()
	r.seedPendingEmailChange(t, id, "new@example.com", token)
	confirm := usecase.NewConfirmEmailChange(r.users)

	err := r.runBlockedBy(t, consumeEmailChange(id, "new@example.com"), func(ctx context.Context) error {
		return confirm.Execute(ctx, token.Plaintext())
	})

	if !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("ConfirmEmailChange.Execute() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
	if got := r.emailChangedEvents(t, id); got != 0 {
		t.Errorf("email changed events = %d, want 0", got)
	}
}

func TestConfirmEmailChangeAppliesPendingEmail(t *testing.T) {
	r := newTestRepos(t)
	id := r.seedUser(t, "old@example.com")
	token := user.NewConfirmationToken()
	r.seedPendingEmailChange(t, id, "new@example.com", token)

	if err := usecase.NewConfirmEmailChange(r.users).Execute(t.Context(), token.Plaintext()); err != nil {
		t.Fatalf("ConfirmEmailChange.Execute() error: %v", err)
	}

	if got := r.count(t, `SELECT count(*) FROM users WHERE id = $1 AND email = 'new@example.com'`, id.String()); got != 1 {
		t.Errorf("users with the new email = %d, want 1", got)
	}
	if got := r.count(t, `SELECT count(*) FROM pending_email_changes WHERE user_id = $1`, id.String()); got != 0 {
		t.Errorf("pending email changes for %v = %d, want 0", id, got)
	}
	if got := r.emailChangedEvents(t, id); got != 1 {
		t.Errorf("email changed events = %d, want 1", got)
	}
}

func TestConfirmEmailChangeRejectsTakenEmail(t *testing.T) {
	r := newTestRepos(t)
	id := r.seedUser(t, "old@example.com")
	token := user.NewConfirmationToken()
	r.seedPendingEmailChange(t, id, "new@example.com", token)
	r.seedUser(t, "new@example.com")
	confirm := usecase.NewConfirmEmailChange(r.users)

	if err := confirm.Execute(t.Context(), token.Plaintext()); !errors.Is(err, user.ErrEmailTaken) {
		t.Errorf("ConfirmEmailChange.Execute() error = %v, want %v", err, user.ErrEmailTaken)
	}

	if got := r.count(t, `SELECT count(*) FROM users WHERE id = $1 AND email = 'old@example.com'`, id.String()); got != 1 {
		t.Errorf("users with the old email = %d, want 1", got)
	}
	if got := r.count(t, `SELECT count(*) FROM pending_email_changes WHERE user_id = $1`, id.String()); got != 0 {
		t.Errorf("pending email changes for %v = %d, want 0", id, got)
	}
	if diff := cmp.Diff(userState{Users: 1, Pending: 0, Events: map[string]int{"user.email_change_rejected": 1}}, r.stateOf(t, id)); diff != "" {
		t.Errorf("after ConfirmEmailChange.Execute(): state of %v mismatch (-want +got):\n%s", id, diff)
	}
	if err := confirm.Execute(t.Context(), token.Plaintext()); !errors.Is(err, user.ErrInvalidConfirmationToken) {
		t.Errorf("repeated ConfirmEmailChange.Execute() error = %v, want %v", err, user.ErrInvalidConfirmationToken)
	}
}

func TestUpdateUnknownUserReturnsNotFound(t *testing.T) {
	r := newTestRepos(t)
	noop := func(*user.User) error { return nil }

	if _, err := r.users.UpdateByID(t.Context(), uuid.New(), noop); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Users.UpdateByID() error = %v, want %v", err, user.ErrNotFound)
	}
	hash := user.NewConfirmationToken().Hash()
	if _, err := r.users.UpdateByEmailChangeToken(t.Context(), hash, noop); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Users.UpdateByEmailChangeToken() error = %v, want %v", err, user.ErrNotFound)
	}
	if _, err := r.users.UpdateByEmail(t.Context(), user.RestoreEmail("nobody@example.com"), noop); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Users.UpdateByEmail() error = %v, want %v", err, user.ErrNotFound)
	}
}

func TestUpdateByEmailSavesChangesAndEvents(t *testing.T) {
	r := newTestRepos(t)
	id := r.seedUser(t, "jane@example.com")

	_, err := r.users.UpdateByEmail(t.Context(), user.RestoreEmail("jane@example.com"), func(u *user.User) error {
		u.RecordDuplicateRegistration()
		u.Rename(testName("Renamed", "User"))
		return nil
	})
	if err != nil {
		t.Fatalf("Users.UpdateByEmail() error: %v", err)
	}

	if got := r.count(t, `SELECT count(*) FROM users WHERE id = $1 AND first_name = 'Renamed'`, id.String()); got != 1 {
		t.Errorf("renamed users = %d, want 1", got)
	}
	if diff := cmp.Diff(userState{Users: 1, Pending: 0, Events: map[string]int{
		"user.duplicate_registration_attempted": 1,
		"user.renamed":                          1,
	}}, r.stateOf(t, id)); diff != "" {
		t.Errorf("after Users.UpdateByEmail(): state of %v mismatch (-want +got):\n%s", id, diff)
	}
}
