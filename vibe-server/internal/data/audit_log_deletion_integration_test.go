//go:build integration

package data

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestIntegration_DeletingAnAccountKeepsItsTrailAndDoesNotFail pins what
// audit_log_user_id_fkey's ON DELETE SET NULL changed, in both directions at once.
//
// audit_log.user_id was declared with no ON DELETE clause, which is NO ACTION:
// the users row could not be deleted while any entry pointed at it. Since
// UserModel.Delete is a plain DELETE FROM users, DELETE /api/v1/auth/me
// answered 500 for every account that had ever been named in the trail — which,
// once internal/auth records sign-outs and password changes, is every account
// that has used the product. The audit trail was blocking the one act it is
// least entitled to block, and nothing in the unit suite could see it, because
// stubs have no foreign keys.
//
// Two assertions, and neither alone is the requirement:
//
//   - the delete succeeds. That is the defect this migration removes.
//   - the entry is still there afterwards, with its action and the account it
//     names intact, and only the actor pointer gone. That is what rules out
//     the other repair: ON DELETE CASCADE would also make the delete succeed,
//     by destroying the record of everything the account ever did. A test that
//     checked only the first would pass against exactly the wrong fix.
func TestIntegration_DeletingAnAccountKeepsItsTrailAndDoesNotFail(t *testing.T) {
	f := newTestFixture(t)
	ctx := context.Background()

	// A user of this test's own, so the shared fixture's account is not the one
	// being destroyed halfway through.
	var userID uuid.UUID
	err := f.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, first_name, last_name, phone, role, email_verified)
		VALUES ($1, 'x', 'Deleted', 'Account', '+5491100009999', 'owner', true)
		RETURNING id`,
		"audit-fk-"+uuid.NewString()+"@test.com",
	).Scan(&userID)
	if err != nil {
		t.Fatalf("creating the account to delete: %v", err)
	}

	var entryID uuid.UUID
	err = f.Pool.QueryRow(ctx, `
		INSERT INTO audit_log (user_id, action, entity_type, entity_id)
		VALUES ($1, 'password_change', 'user', $1)
		RETURNING id`, userID,
	).Scan(&entryID)
	if err != nil {
		t.Fatalf("writing the audit entry: %v", err)
	}
	t.Cleanup(func() {
		if _, err := f.Pool.Exec(context.Background(), `DELETE FROM audit_log WHERE id = $1`, entryID); err != nil {
			t.Errorf("cleaning up the audit entry: %v", err)
		}
	})

	if err := f.Models.Users.Delete(ctx, userID); err != nil {
		t.Fatalf("deleting an account that has an audit entry: %v\n"+
			"audit_log.user_id must be ON DELETE SET NULL; a NO ACTION reference makes "+
			"the trail refuse the deletion of any account it has ever named", err)
	}

	var (
		survivingUserID *uuid.UUID
		action          string
		entity          uuid.UUID
	)
	err = f.Pool.QueryRow(ctx,
		`SELECT user_id, action, entity_id FROM audit_log WHERE id = $1`, entryID,
	).Scan(&survivingUserID, &action, &entity)
	if err != nil {
		t.Fatalf("the entry did not survive the account it named: %v\n"+
			"ON DELETE CASCADE would produce exactly this — the delete succeeds and the "+
			"record of what the account did goes with it", err)
	}

	if survivingUserID != nil {
		t.Errorf("audit_log.user_id = %v after the account was deleted; want NULL", *survivingUserID)
	}
	if action != "password_change" {
		t.Errorf("audit_log.action = %q after the deletion; want it untouched", action)
	}
	if entity != userID {
		t.Errorf("audit_log.entity_id = %v; want %v — entity_id carries no foreign key and is "+
			"what still identifies the account after user_id is cleared", entity, userID)
	}
}
