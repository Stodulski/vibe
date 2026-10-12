//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	complexstore "github.com/stodulski/vibe-server/internal/complexes/store"
	"github.com/stodulski/vibe-server/internal/data"
	datatest "github.com/stodulski/vibe-server/internal/data/datatest"
)

// fixtureSlugAndCity reads the slug, name and city of the fixture's complex.
func fixtureSlugAndCity(ctx context.Context, t *testing.T, f *datatest.Fixture) (slug, name, city string) {
	t.Helper()
	if err := f.DB.QueryRow(ctx,
		`SELECT slug, name, city FROM complexes WHERE id = $1`, f.ComplexID,
	).Scan(&slug, &name, &city); err != nil {
		t.Fatalf("reading the fixture's slug: %v", err)
	}
	return slug, name, city
}

// insertComplexWithSlug inserts a live complex under the given owner. Each call
// takes a fresh owner (see insertComplexFor), because one owner holds one live
// complex and that index must not get in the way of a slug test.
func insertComplexWithSlug(ctx context.Context, f *datatest.Fixture, owner uuid.UUID, name, city, slug string) (uuid.UUID, error) {
	c := &complexstore.Complex{
		OwnerID:           owner,
		Name:              name,
		Slug:              slug,
		Address:           "Av. Siempreviva 742",
		City:              city,
		Province:          "Santa Cruz",
		CountryCode:       "AR",
		Currency:          "ARS",
		Phone:             "+5491100000042",
		CancellationHours: 24,
		Amenities:         []string{},
	}
	if err := f.Stores.Complexes.Insert(ctx, c); err != nil {
		return uuid.Nil, err
	}
	return c.ID, nil
}

// A deleted complex frees its slug: the public URL can name a new venue, and
// the availability check and the insert agree that it is free.
func TestIntegration_ADeletedComplexFreesItsSlugForANewComplex(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	slug, _, _ := fixtureSlugAndCity(ctx, t, f)

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}

	taken, err := f.Stores.Complexes.SlugExists(ctx, slug)
	if err != nil {
		t.Fatalf("SlugExists: %v", err)
	}
	if taken {
		t.Errorf("SlugExists reports the slug of a deleted complex as taken")
	}

	if _, err := insertComplexWithSlug(ctx, f, f.InsertOwner(t), "Club Nuevo", "Rosario", slug); err != nil {
		t.Fatalf("a new complex must be able to take a deleted complex's slug: %v", err)
	}
}

// Two live complexes still cannot share a slug: the database answers with the
// same sentinel the store has always returned for that conflict.
func TestIntegration_TwoLiveComplexesStillCannotShareASlug(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	slug, _, _ := fixtureSlugAndCity(ctx, t, f)

	_, err := insertComplexWithSlug(ctx, f, f.InsertOwner(t), "Club Copia", "Rosario", slug)
	if !errors.Is(err, complexstore.ErrDuplicateSlug) {
		t.Fatalf("a second live complex on a live slug: want ErrDuplicateSlug, got %v", err)
	}
}

// The slug helpers count live complexes only, so a suggestion never names a
// variant that is free, and a deleted venue's slug is never reported as the
// one in use.
func TestIntegration_SlugHelpersSeeOnlyLiveComplexes(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	slug, _, _ := fixtureSlugAndCity(ctx, t, f)

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}

	variants, err := f.Stores.Complexes.SlugsWithPrefix(ctx, slug)
	if err != nil {
		t.Fatalf("SlugsWithPrefix: %v", err)
	}
	for _, v := range variants {
		if v == slug {
			t.Errorf("SlugsWithPrefix still lists the deleted complex's slug %q", slug)
		}
	}
}

// The 410 lookup names the most recently deleted complex that held the slug,
// and answers nothing while a live complex holds it or when no complex ever did.
func TestIntegration_GetGoneBySlugNamesTheLastDeletedHolder(t *testing.T) {
	f := datatest.Isolated(t)
	ctx := context.Background()
	slug, name, city := fixtureSlugAndCity(ctx, t, f)

	if _, err := f.Stores.Complexes.GetGoneBySlug(ctx, slug); !errors.Is(err, data.ErrRecordNotFound) {
		t.Fatalf("a live slug is not gone: want ErrRecordNotFound, got %v", err)
	}

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, f.ComplexID); err != nil {
		t.Fatalf("SoftDeleteCascade: %v", err)
	}
	gone, err := f.Stores.Complexes.GetGoneBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetGoneBySlug after the delete: %v", err)
	}
	if gone.Name != name || gone.City != city {
		t.Errorf("GetGoneBySlug = %+v; want the deleted complex's name %q and city %q", gone, name, city)
	}

	// The fixture runs every statement in one transaction, and NOW() is that
	// transaction's start time, so both deletes below would carry the same
	// deleted_at. Push the first one back so "most recently deleted" is decided
	// by time, as it is for two requests in production.
	if _, err := f.DB.Exec(ctx,
		`UPDATE complexes SET deleted_at = NOW() - interval '1 hour' WHERE id = $1`, f.ComplexID); err != nil {
		t.Fatalf("backdating the first delete: %v", err)
	}

	newID, err := insertComplexWithSlug(ctx, f, f.InsertOwner(t), "Club Nuevo", "Rosario", slug)
	if err != nil {
		t.Fatalf("insert the new complex: %v", err)
	}
	if _, err := f.Stores.Complexes.GetGoneBySlug(ctx, slug); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a slug a live complex holds is not gone: want ErrRecordNotFound, got %v", err)
	}

	if _, err := f.Stores.Complexes.SoftDeleteCascade(ctx, newID); err != nil {
		t.Fatalf("SoftDeleteCascade of the new complex: %v", err)
	}
	gone, err = f.Stores.Complexes.GetGoneBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetGoneBySlug after the second delete: %v", err)
	}
	if gone.Name != "Club Nuevo" || gone.City != "Rosario" {
		t.Errorf("GetGoneBySlug = %+v; want the most recently deleted holder, Club Nuevo in Rosario", gone)
	}

	if _, err := f.Stores.Complexes.GetGoneBySlug(ctx, "nunca-existio"); !errors.Is(err, data.ErrRecordNotFound) {
		t.Errorf("a slug no complex ever held: want ErrRecordNotFound, got %v", err)
	}
}
