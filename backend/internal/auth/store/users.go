package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/db"
)

// User represents an authenticated account, either a complex owner or a platform admin.
type User struct {
	ID                  uuid.UUID  `json:"id"`
	Email               string     `json:"email"`
	PasswordHash        []byte     `json:"-"`
	FirstName           string     `json:"first_name"`
	LastName            string     `json:"last_name"`
	Phone               string     `json:"phone"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	EmailVerified       bool       `json:"email_verified"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	FailedLoginAttempts int        `json:"-"`
	LockedUntil         *time.Time `json:"-"`
	LastFailedLogin     *time.Time `json:"-"`
}

// DefaultHashCost is the bcrypt cost SetPassword hashes with when its caller
// names none. 12 is the production value: about a quarter of a second per hash
// on current hardware, which is the point of it. Test binaries pass
// bcrypt.MinCost instead — through the configuration of the handler that hashes,
// not through a package variable — because the same hash under the race
// detector takes nine seconds and the suites that register or sign in users
// paid it hundreds of times per run.
const DefaultHashCost = 12

// hashCost is the given cost, or DefaultHashCost when none was named.
func hashCost(cost int) int {
	if cost <= 0 {
		return DefaultHashCost
	}
	return cost
}

// SetPassword hashes the plaintext password with bcrypt at the given cost and
// stores it on the user. A cost of zero means DefaultHashCost.
//
// The cost is a parameter rather than a package variable because a package
// variable is writable from anywhere, and was in practice written by three
// separate TestMains. The caller that knows which binary this is — the auth
// handler, from its own Config — is the one that decides (CON-07).
func (u *User) SetPassword(plain string, cost int) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), hashCost(cost))
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}
	u.PasswordHash = hash
	return nil
}

// PasswordMatches reports whether plain matches the user's stored password hash.
func (u *User) PasswordMatches(plain string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(plain))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, fmt.Errorf("auth: compare password: %w", err)
	}
	return true, nil
}

// IsLocked returns true if the user account is currently locked due to failed login attempts.
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// ComparePassword is a package-level helper for timing-safe dummy comparisons.
func ComparePassword(hash []byte, plain string) error {
	if err := bcrypt.CompareHashAndPassword(hash, []byte(plain)); err != nil {
		return fmt.Errorf("auth: compare password: %w", err)
	}
	return nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// DummyPasswordHash is a real bcrypt hash at the given cost, for the sign-in
// path to compare against when the account does not exist, so that a miss
// costs the same as a wrong password. It is hashed once, on first use, and at
// the caller's cost rather than at a literal one: a test binary that lowered
// the cost must not pay a cost-12 comparison for every failed sign-in either.
// A cost of zero means DefaultHashCost, and the first caller's cost is the one
// the process keeps.
func DummyPasswordHash(cost int) []byte {
	dummyHashOnce.Do(func() {
		hash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-password"), hashCost(cost))
		if err != nil {
			panic("authstore: hashing the dummy password: " + err.Error())
		}
		dummyHash = hash
	})
	return dummyHash
}

// Users implements UserStore against PostgreSQL.
type Users struct {
	DB *data.DB
	Q  *db.Queries
	// HashCost is the bcrypt cost this store's callers hash at. Zero means
	// DefaultHashCost. stores.New sets it from application configuration; a
	// test sets it on its own fixture.
	HashCost int
}

// Insert creates a new user, returning ErrDuplicateEmail if the email is already registered.
func (m *Users) Insert(ctx context.Context, user *User) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbUser, err := m.Q.InsertUser(ctx, db.InsertUserParams{
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Phone:        user.Phone,
		Role:         db.UserRole(user.Role),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		return err
	}

	user.ID = data.PgToUUID(dbUser.ID)
	user.IsActive = dbUser.IsActive
	user.EmailVerified = dbUser.EmailVerified
	user.CreatedAt = data.PgToTime(dbUser.CreatedAt)
	user.UpdatedAt = data.PgToTime(dbUser.UpdatedAt)
	return nil
}

// GetByEmail returns the user with the given email, or ErrRecordNotFound if none exists.
//
// It goes through the generated GetUserByEmail and userFromDB rather than the
// hand-written SELECT and Scan it used to carry. That second field list was the
// mechanism by which the lockout columns went missing from GetByID and nobody
// noticed: the login path read them, so IsLocked worked, so the mapper that
// dropped them looked correct. One query, one mapper, one field list — a column
// is now either loaded for every caller or for none, and there is no longer a
// pair of lists that can silently disagree.
func (m *Users) GetByEmail(ctx context.Context, email string) (*User, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbUser, err := m.Q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return userFromDB(dbUser), nil
}

// GetByID returns the user with the given ID, or ErrRecordNotFound if none exists.
func (m *Users) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbUser, err := m.Q.GetUserByID(ctx, data.UUIDToPg(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, data.ErrRecordNotFound
		}
		return nil, err
	}
	return userFromDB(dbUser), nil
}

// Update persists changes to an existing user, returning ErrRecordNotFound if it no longer
// exists or ErrDuplicateEmail if the new email is already registered to another user.
func (m *Users) Update(ctx context.Context, user *User) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	dbUser, err := m.Q.UpdateUser(ctx, db.UpdateUserParams{
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Phone:     user.Phone,
		IsActive:  user.IsActive,
		// Written, not just held in memory: changing the email address must clear
		// verification, or the account moves onto an address nobody proved they own
		// and password-reset links follow it there.
		EmailVerified: user.EmailVerified,
		ID:            data.UUIDToPg(user.ID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return data.ErrRecordNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		return err
	}

	user.UpdatedAt = data.PgToTime(dbUser.UpdatedAt)
	return nil
}

// UpdatePassword replaces the user's stored password hash, returning ErrRecordNotFound if the user no longer exists.
func (m *Users) UpdatePassword(ctx context.Context, userID uuid.UUID, newHash []byte) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	result, err := m.DB.Exec(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2", newHash, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return data.ErrRecordNotFound
	}
	return nil
}

// SetEmailVerified marks the user's email address as verified.
func (m *Users) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.SetEmailVerified(ctx, data.UUIDToPg(userID))
}

// Delete permanently removes the user account.
func (m *Users) Delete(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.DeleteUser(ctx, data.UUIDToPg(userID))
}

// DeleteUnverifiedStale deletes accounts that never verified their email within the retention window.
func (m *Users) DeleteUnverifiedStale(ctx context.Context) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	return m.Q.DeleteUnverifiedStaleUsers(ctx)
}

// IncrementFailedAttempts atomically increments the failed login counter and
// applies progressive lockout: 5 attempts = 15 min, 10 = 1 hour, 15+ = 4 hours.
func (m *Users) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    last_failed_login = NOW(),
		    locked_until = CASE
		        WHEN failed_login_attempts + 1 >= 15 THEN NOW() + INTERVAL '4 hours'
		        WHEN failed_login_attempts + 1 >= 10 THEN NOW() + INTERVAL '1 hour'
		        WHEN failed_login_attempts + 1 >= 5  THEN NOW() + INTERVAL '15 minutes'
		        ELSE locked_until
		    END
		WHERE id = $1`, userID)
	return err
}

// ResetFailedAttempts clears the failed login counter and lockout after a successful login.
func (m *Users) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := data.QueryContext(ctx)
	defer cancel()

	_, err := m.DB.Exec(ctx, `
		UPDATE users
		SET failed_login_attempts = 0, locked_until = NULL, last_failed_login = NULL
		WHERE id = $1`, userID)
	return err
}

// userFromDB is the one and only translation from a users row to a domain User.
//
// It carries every column, including the three lockout columns. Those were
// dropped for a while, which was survivable only by accident: IsLocked had a
// single caller, Login, and Login loaded its user through a second reader that
// happened to populate them. Any lockout check on a user loaded any other way —
// the refresh handler, the password-reset handler, the authenticate middleware —
// would have read LockedUntil as nil and concluded "not locked". A security
// check that answers "no" because a field was never filled in fails open and
// returns no error while doing it.
//
// The reason it is the *only* mapper is the actual fix. There used to be two
// readers of this table with two hand-written field lists, and the guarantee
// that they agreed was that somebody remembered. TestUserFromDBCarriesEveryColumn
// is the other half: it fails when db.User grows a column this function does not
// carry, so the next field cannot be dropped quietly the way these three were.
func userFromDB(u db.User) *User {
	return &User{
		ID:                  data.PgToUUID(u.ID),
		Email:               u.Email,
		PasswordHash:        u.PasswordHash,
		FirstName:           u.FirstName,
		LastName:            u.LastName,
		Phone:               u.Phone,
		Role:                string(u.Role),
		IsActive:            u.IsActive,
		EmailVerified:       u.EmailVerified,
		CreatedAt:           data.PgToTime(u.CreatedAt),
		UpdatedAt:           data.PgToTime(u.UpdatedAt),
		FailedLoginAttempts: int(u.FailedLoginAttempts),
		LockedUntil:         data.PgToTimePtr(u.LockedUntil),
		LastFailedLogin:     data.PgToTimePtr(u.LastFailedLogin),
	}
}
