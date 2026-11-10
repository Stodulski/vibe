package clients

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Owner rule (2026-11-10, verbatim): "hay que ver que no se bloqueen clientes
// sin razon". Blocking a client is the owner's manual edit and nothing else:
// no booking, payment, expiry or cancellation path may write is_blocked. The
// payment paths are pinned at runtime by the payments store integration tests
// (attach_client_integration_test.go); this pins the source, so a new writer
// anywhere in the backend has to be added here on purpose.
func TestOnlyTheOwnersClientEditWritesIsBlocked(t *testing.T) {
	root := filepath.Join("..", "..")
	sqlWrite := regexp.MustCompile(`(?i)\bis_blocked\s*=[^=]`)
	// Go files are checked for both: SQL lives in their raw strings too.
	goWrite := regexp.MustCompile(`\.IsBlocked\s*=[^=]|IsBlocked:\s*(true|!)|(?i)\bis_blocked\s*=[^=]`)

	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			// Generated code mirrors db/queries; migrations are schema.
			if rel == filepath.Join("internal", "db") || rel == filepath.Join("internal", "openapi", "gen") ||
				rel == filepath.Join("db", "migrations") || (rel != "." && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		var re *regexp.Regexp
		switch {
		case strings.HasSuffix(path, "_test.go"):
			return nil
		case strings.HasSuffix(path, ".go"):
			re = goWrite
		case strings.HasSuffix(path, ".sql"):
			re = sqlWrite
		default:
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "--") {
				continue
			}
			if re.MatchString(line) {
				found = append(found, filepath.ToSlash(rel)+": "+trimmed)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		// UpdateClient, the owner's client edit.
		"db/queries/clients.sql: is_blocked = $6",
		// Service.Update applying the owner's is_blocked field.
		"internal/clients/service.go: client.IsBlocked = *in.IsBlocked",
		// Store.Insert reading the column's default back into memory.
		"internal/clients/store/clients.go: c.IsBlocked = dbClient.IsBlocked",
	}
	slices.Sort(found)
	if !slices.Equal(found, want) {
		t.Errorf("is_blocked is written outside the owner's client edit.\nwant only:\n  %s\ngot:\n  %s",
			strings.Join(want, "\n  "), strings.Join(found, "\n  "))
	}
}
