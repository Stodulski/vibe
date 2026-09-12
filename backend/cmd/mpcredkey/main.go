// Command mpcredkey is a standalone operator tool for converting MercadoPago
// OAuth credentials between plaintext and the v1 encryption envelope
// (internal/crypto). It exists for developer and E2E databases only — see
// design.md's Migration/Rollout section: nothing is deployed yet, so there
// is no production ciphertext to convert or roll forward, only seed and
// test data.
//
// Usage:
//
//	mpcredkey seal  -db-dsn=<dsn> -mp-credential-keys=kid:base64key[,...]
//	mpcredkey rekey -db-dsn=<dsn> -mp-credential-keys=kid:base64key[,...]
//
// seal converts plaintext mp_access_token/mp_refresh_token values to the v1
// envelope, sealed under the keyring's active (first) key.
//
// rekey re-seals every v1 envelope under the keyring's active key. Any
// entry in the keyring can open an existing envelope — that is what makes
// key rotation possible: prepend the new key, run rekey, then drop the
// retired key on the next deploy.
//
// Both subcommands are idempotent: seal only touches values that are not
// already envelope-shaped, and rekey only touches values that already are,
// so running either one twice (or running the wrong one) converts nothing
// on the second pass.
//
// Precedent: cmd/benchmark, the other standalone operator script that talks
// to the database directly with pgxpool rather than through cmd/api.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stodulski/vibe-server/internal/crypto"
	"github.com/stodulski/vibe-server/internal/mpcred"
)

const usage = "usage: mpcredkey <seal|rekey> -db-dsn=<dsn> -mp-credential-keys=kid:base64key[,...]"

func main() {
	if len(os.Args) < 2 {
		log.Fatal(usage)
	}
	mode := os.Args[1]
	if mode != "seal" && mode != "rekey" {
		//nolint:gosec // G706: mode is CLI-operator-controlled argv on a local admin
		// tool, not untrusted network input.
		log.Fatalf("unknown subcommand %q\n%s", mode, usage)
	}

	dsn, keysSpec := parseFlags(mode)

	keyring, err := crypto.ParseKeyring(keysSpec)
	if err != nil {
		log.Fatalf("parsing keyring: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		cancel()
		//nolint:gocritic // exitAfterDefer: the deferred cancel() is already replicated
		// explicitly on the line above, so this log.Fatalf does not actually skip it.
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	converted, err := convertCredentials(ctx, pool, keyring, mode)
	if err != nil {
		//nolint:gosec // G706: mode is CLI-operator-controlled argv on a local admin
		// tool, not untrusted network input.
		log.Fatalf("%s: %v", mode, err)
	}
	fmt.Printf("%s: converted %d complex(es)\n", mode, converted)
}

// parseFlags parses the subcommand's flags, falling back to DATABASE_URL /
// MP_CREDENTIAL_KEYS when a flag is not given — the same flag-first,
// env-override precedent cmd/api/main.go uses, mirrored here for an
// operator invoking this from the same environment a deploy runs in.
func parseFlags(mode string) (dsn, keysSpec string) {
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	dsnFlag := fs.String("db-dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN (defaults to DATABASE_URL)")
	keysFlag := fs.String("mp-credential-keys", os.Getenv("MP_CREDENTIAL_KEYS"),
		"kid:base64key[,kid:base64key...] (defaults to MP_CREDENTIAL_KEYS)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("parsing flags: %v", err)
	}

	if *dsnFlag == "" {
		log.Fatal("db-dsn flag or DATABASE_URL env var must be set")
	}
	return *dsnFlag, *keysFlag
}

// credentialRow is one complex's raw mp_access_token / mp_refresh_token
// columns, read straight from the database — never through
// internal/data.ComplexModel, whose accessors would refuse to hand back
// ciphertext this tool needs to read and rewrite directly.
type credentialRow struct {
	id           uuid.UUID
	accessToken  pgtype.Text
	refreshToken pgtype.Text
}

// convertCredentials walks every complex holding a MercadoPago credential in
// either column and converts each according to mode. A complex whose columns
// are both already in the target shape is read but not written.
func convertCredentials(ctx context.Context, pool *pgxpool.Pool, keyring *crypto.Keyring, mode string) (int, error) {
	rows, err := fetchCredentialRows(ctx, pool)
	if err != nil {
		return 0, fmt.Errorf("reading credential rows: %w", err)
	}

	converted := 0
	for _, row := range rows {
		newAccess, accessChanged, err := convertColumn(keyring, row.id, mpcred.AccessTokenColumn, row.accessToken, mode)
		if err != nil {
			return converted, fmt.Errorf("complex %s: %s mp_access_token: %w", row.id, mode, err)
		}
		newRefresh, refreshChanged, err := convertColumn(keyring, row.id, mpcred.RefreshTokenColumn, row.refreshToken, mode)
		if err != nil {
			return converted, fmt.Errorf("complex %s: %s mp_refresh_token: %w", row.id, mode, err)
		}
		if !accessChanged && !refreshChanged {
			continue
		}
		if err := writeCredentialRow(ctx, pool, row.id, newAccess, newRefresh); err != nil {
			return converted, fmt.Errorf("complex %s: writing converted credentials: %w", row.id, err)
		}
		converted++
	}
	return converted, nil
}

// convertColumn converts a single stored value according to mode. seal only
// touches a value that is not already envelope-shaped; rekey only touches a
// value that already is. A NULL or empty column is left untouched either
// way — there is nothing to convert.
func convertColumn(keyring *crypto.Keyring, complexID uuid.UUID, column string, value pgtype.Text, mode string) (pgtype.Text, bool, error) {
	if !value.Valid || value.String == "" {
		return value, false, nil
	}

	aad := mpcred.AAD(complexID, column)

	switch {
	case mode == "seal" && !isEnvelope(value.String):
		sealed, err := keyring.Seal(aad, value.String)
		if err != nil {
			return pgtype.Text{}, false, err
		}
		return pgtype.Text{String: sealed, Valid: true}, true, nil

	case mode == "rekey" && isEnvelope(value.String) && !alreadyUnderActiveKey(keyring, value.String):
		plain, err := keyring.Open(aad, value.String)
		if err != nil {
			return pgtype.Text{}, false, err
		}
		sealed, err := keyring.Seal(aad, plain)
		if err != nil {
			return pgtype.Text{}, false, err
		}
		return pgtype.Text{String: sealed, Valid: true}, true, nil

	default:
		// seal on an already-sealed value, rekey on a plaintext one, or
		// rekey on a value already sealed under the active key: not this
		// subcommand's job, left untouched.
		return value, false, nil
	}
}

// alreadyUnderActiveKey reports whether an envelope's kid already matches
// the keyring's active (writing) key, so rekey can skip re-sealing it. Every
// Seal call draws a fresh nonce, so without this check rekey would rewrite
// every row on every run even when nothing needs rotating.
func alreadyUnderActiveKey(keyring *crypto.Keyring, envelope string) bool {
	kid, ok := envelopeKeyID(envelope)
	return ok && kid == keyring.ActiveKeyID()
}

// envelopeKeyID returns the kid segment of a v1 envelope's
// "v1.<kid>.<payload>" shape. ok is false when value is not envelope-shaped.
func envelopeKeyID(value string) (kid string, ok bool) {
	parts := strings.SplitN(value, ".", 3)
	if len(parts) != 3 || parts[0] != "v1" {
		return "", false
	}
	return parts[1], true
}

// isEnvelope reports whether value is already shaped like a v1 credential
// envelope ("v1.<kid>.<payload>"), as opposed to plaintext.
func isEnvelope(value string) bool {
	_, ok := envelopeKeyID(value)
	return ok
}

// fetchCredentialRows reads every complex with a non-null credential in
// either column. A complex with neither set is never walked — there is
// nothing to convert.
func fetchCredentialRows(ctx context.Context, pool *pgxpool.Pool) ([]credentialRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, mp_access_token, mp_refresh_token FROM complexes
		 WHERE mp_access_token IS NOT NULL OR mp_refresh_token IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []credentialRow
	for rows.Next() {
		var row credentialRow
		if err := rows.Scan(&row.id, &row.accessToken, &row.refreshToken); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// writeCredentialRow persists converted access/refresh values. The caller
// passes the original value back unchanged for a column convertColumn did
// not touch, so this is always a plain two-column UPDATE.
func writeCredentialRow(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, access, refresh pgtype.Text) error {
	_, err := pool.Exec(ctx,
		`UPDATE complexes SET mp_access_token = $1, mp_refresh_token = $2 WHERE id = $3`,
		access, refresh, id)
	return err
}
