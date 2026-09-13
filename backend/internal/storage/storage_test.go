package storage

import (
	"context"
	"testing"
	"time"
)

// TestObjectStorage_InterfaceDefinition verifies that ObjectStorage defines the
// expected method signatures. This is a compile-time check.
func TestObjectStorage_InterfaceDefinition(t *testing.T) {
	// Verify the interface is correctly defined by ensuring a mock
	// implementing all methods satisfies the interface at compile time.
	var _ ObjectStorage = (*mockStorage)(nil)
}

// mockStorage implements ObjectStorage for compile-time verification.
type mockStorage struct{}

func (m *mockStorage) GeneratePresignedPUT(_ context.Context, _, _ string, _ int64, _ time.Duration) (string, string, error) {
	return "", "", nil
}

func (m *mockStorage) PutObject(_ context.Context, _ Object) error {
	return nil
}

func (m *mockStorage) GeneratePresignedGET(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "", nil
}

func (m *mockStorage) DeleteObject(_ context.Context, _ string) error {
	return nil
}

func (m *mockStorage) KeyFromPublicURL(_ string) (string, bool) {
	return "", false
}

// TestObjectStorage_MethodSignatures verifies the interface methods
// have the expected parameter and return types by exercising the mock.
func TestObjectStorage_MethodSignatures(t *testing.T) {
	var store ObjectStorage = &mockStorage{}

	t.Run("GeneratePresignedPUT", func(t *testing.T) {
		uploadURL, publicURL, err := store.GeneratePresignedPUT(
			context.Background(),
			"test/key.webp",
			"image/webp",
			5*1024*1024,
			15*time.Minute,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Mock returns empty strings, just verify the call works.
		_ = uploadURL
		_ = publicURL
	})

	t.Run("PutObject", func(t *testing.T) {
		err := store.PutObject(context.Background(), Object{
			Key:                "exports/complex/export.xlsx",
			ContentType:        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			ContentDisposition: `attachment; filename="pagos.xlsx"`,
			Body:               []byte("PK"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("GeneratePresignedGET", func(t *testing.T) {
		downloadURL, err := store.GeneratePresignedGET(
			context.Background(),
			"exports/complex/export.xlsx",
			"pagos.xlsx",
			15*time.Minute,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Mock returns an empty string, just verify the call works.
		_ = downloadURL
	})

	t.Run("DeleteObject", func(t *testing.T) {
		err := store.DeleteObject(context.Background(), "test/key.webp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("KeyFromPublicURL", func(t *testing.T) {
		key, ok := store.KeyFromPublicURL("https://cdn.example.com/test.webp")
		if ok {
			t.Error("mock should return ok=false")
		}
		if key != "" {
			t.Errorf("mock should return empty key, got %q", key)
		}
	})
}
