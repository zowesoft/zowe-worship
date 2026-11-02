package settings

import (
	"context"
	"errors"
	"testing"
)

func TestGetSetDelete(t *testing.T) {
	db := openTestDB(t)
	store := New(db)
	ctx := context.Background()

	// Missing key returns ErrNotFound.
	if _, err := store.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing key: want ErrNotFound, got %v", err)
	}

	// GetDefault returns the fallback for a missing key.
	if v := store.GetDefault(ctx, "missing", "fallback"); v != "fallback" {
		t.Fatalf("GetDefault missing = %q, want %q", v, "fallback")
	}

	// Set then Get.
	if err := store.Set(ctx, "mykey", "myvalue"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, err := store.Get(ctx, "mykey")
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if v != "myvalue" {
		t.Fatalf("Get = %q, want %q", v, "myvalue")
	}

	// Set overwrites.
	if err := store.Set(ctx, "mykey", "updated"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if v, _ := store.Get(ctx, "mykey"); v != "updated" {
		t.Fatalf("Get after overwrite = %q, want %q", v, "updated")
	}

	// Delete.
	if err := store.Delete(ctx, "mykey"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, "mykey"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: want ErrNotFound, got %v", err)
	}

	// Deleting a missing key is not an error.
	if err := store.Delete(ctx, "never-existed"); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestKnownKeys(t *testing.T) {
	// Verify constants exist and are non-empty — catches accidental blanking.
	keys := []string{KeyDefaultTranslation}
	for _, k := range keys {
		if k == "" {
			t.Fatalf("known key constant is empty")
		}
	}
}
