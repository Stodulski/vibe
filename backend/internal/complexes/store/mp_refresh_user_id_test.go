package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stodulski/vibe-server/internal/data"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

// recordingRunner keeps every statement sent to it and answers each as done.
type recordingRunner struct {
	sql  []string
	args [][]any
}

func (r *recordingRunner) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.sql, r.args = append(r.sql, sql), append(r.args, args)
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (r *recordingRunner) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not used")
}

func (r *recordingRunner) QueryRow(context.Context, string, ...any) pgx.Row { return nil }

// A refresh answer without user_id must still store the rotated tokens: the old
// refresh token is spent, and refusing the write left the venue with a dead
// credential. The write then leaves mp_user_id as it is instead of storing an
// empty or invented id; a refresh that names the account replaces whatever
// was stored, including a "0" written before connect and refresh refused it.
func TestUpdateMPCredentialsWithoutAUserIDKeepsTheStoredOne(t *testing.T) {
	r := &recordingRunner{}
	m := &Store{DB: data.NewDBOver(r, func(context.Context, time.Duration) error { return nil }), Keys: testKeyring32(t, "k1", 0x05)}

	if err := m.UpdateMPCredentials(t.Context(), uuid.New(), "access", "refresh", "", 3600); err != nil {
		t.Fatalf("a refresh without user_id must still store the tokens; got %v", err)
	}
	if err := m.UpdateMPCredentials(t.Context(), uuid.New(), "access", "refresh", "42", 3600); err != nil {
		t.Fatalf("UpdateMPCredentials: %v", err)
	}

	if len(r.sql) != 2 {
		t.Fatalf("want two writes; got %d", len(r.sql))
	}
	if !strings.Contains(r.sql[0], "COALESCE(NULLIF($3, ''), mp_user_id)") {
		t.Errorf("an empty user id must leave mp_user_id as stored; got %s", r.sql[0])
	}
	if got := r.args[1][2]; got != "42" {
		t.Errorf("a named user id must be written; got %v", got)
	}
}

// Connecting is the one write that must name the account: with no user id the
// venue would be connected to nobody.
func TestConnectMPCredentialsRefusesAnEmptyUserID(t *testing.T) {
	m := &Store{Keys: testKeyring32(t, "k1", 0x05)}
	err := m.ConnectMPCredentials(t.Context(), uuid.New(), "access", "refresh", "", 0)
	if !errors.Is(err, mpcred.ErrMPCredentialEmpty) {
		t.Errorf("want mpcred.ErrMPCredentialEmpty; got %v", err)
	}
}
