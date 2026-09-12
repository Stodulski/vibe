package data

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

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

// PasswordHashCost is the bcrypt cost SetPassword hashes with. 12 is the
// production value: about a quarter of a second per hash on current hardware,
// which is the point of it. Test binaries lower it to bcrypt.MinCost from
// their TestMain, because the same hash under the race detector takes nine
// seconds and the suites that register or sign in users paid it hundreds of
// times per run. Nothing outside a test may change it.
var PasswordHashCost = 12

// SetPassword hashes the plaintext password with bcrypt and stores it on the user.
func (u *User) SetPassword(plain string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), PasswordHashCost)
	if err != nil {
		return err
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
		return false, err
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
	return bcrypt.CompareHashAndPassword(hash, []byte(plain))
}

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// DummyPasswordHash is a real bcrypt hash at PasswordHashCost, for the sign-in
// path to compare against when the account does not exist, so that a miss
// costs the same as a wrong password. It is hashed once, on first use, and at
// the configured cost rather than a literal one: a test binary that lowered
// the cost must not pay a cost-12 comparison for every failed sign-in either.
func DummyPasswordHash() []byte {
	dummyHashOnce.Do(func() {
		hash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-password"), PasswordHashCost)
		if err != nil {
			panic("data: hashing the dummy password: " + err.Error())
		}
		dummyHash = hash
	})
	return dummyHash
}

// UserModel implements UserStore against PostgreSQL.
type UserModel struct {
	DB *DB
	Q  *db.Queries
}

// Insert creates a new user, returning ErrDuplicateEmail if the email is already registered.
func (m *UserModel) Insert(ctx context.Context, user *User) error {
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

	user.ID = PgToUUID(dbUser.ID)
	user.IsActive = dbUser.IsActive
	user.EmailVerified = dbUser.EmailVerified
	user.CreatedAt = PgToTime(dbUser.CreatedAt)
	user.UpdatedAt = PgToTime(dbUser.UpdatedAt)
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
func (m *UserModel) GetByEmail(ctx context.Context, email string) (*User, error) {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	dbUser, err := m.Q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return userFromDB(dbUser), nil
}

// GetByID returns the user with the given ID, or ErrRecordNotFound if none exists.
func (m *UserModel) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	dbUser, err := m.Q.GetUserByID(ctx, UUIDToPg(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return userFromDB(dbUser), nil
}

// Update persists changes to an existing user, returning ErrRecordNotFound if it no longer
// exists or ErrDuplicateEmail if the new email is already registered to another user.
func (m *UserModel) Update(ctx context.Context, user *User) error {
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
		ID:            UUIDToPg(user.ID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRecordNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		return err
	}

	user.UpdatedAt = PgToTime(dbUser.UpdatedAt)
	return nil
}

// UpdatePassword replaces the user's stored password hash, returning ErrRecordNotFound if the user no longer exists.
func (m *UserModel) UpdatePassword(ctx context.Context, userID uuid.UUID, newHash []byte) error {
	ctx, cancel := QueryContext(ctx)
	defer cancel()

	result, err := m.DB.Exec(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2", newHash, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// SetEmailVerified marks the user's email address as verified.
func (m *UserModel) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	return m.Q.SetEmailVerified(ctx, UUIDToPg(userID))
}

// Delete permanently removes the user account.
func (m *UserModel) Delete(ctx context.Context, userID uuid.UUID) error {
	return m.Q.DeleteUser(ctx, UUIDToPg(userID))
}

// DeleteUnverifiedStale deletes accounts that never verified their email within the retention window.
func (m *UserModel) DeleteUnverifiedStale(ctx context.Context) error {
	return m.Q.DeleteUnverifiedStaleUsers(ctx)
}

// IncrementFailedAttempts atomically increments the failed login counter and
// applies progressive lockout: 5 attempts = 15 min, 10 = 1 hour, 15+ = 4 hours.
func (m *UserModel) IncrementFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := QueryContext(ctx)
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
func (m *UserModel) ResetFailedAttempts(ctx context.Context, userID uuid.UUID) error {
	ctx, cancel := QueryContext(ctx)
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
		ID:                  PgToUUID(u.ID),
		Email:               u.Email,
		PasswordHash:        u.PasswordHash,
		FirstName:           u.FirstName,
		LastName:            u.LastName,
		Phone:               u.Phone,
		Role:                string(u.Role),
		IsActive:            u.IsActive,
		EmailVerified:       u.EmailVerified,
		CreatedAt:           PgToTime(u.CreatedAt),
		UpdatedAt:           PgToTime(u.UpdatedAt),
		FailedLoginAttempts: int(u.FailedLoginAttempts),
		LockedUntil:         PgToTimePtr(u.LockedUntil),
		LastFailedLogin:     PgToTimePtr(u.LastFailedLogin),
	}
}
