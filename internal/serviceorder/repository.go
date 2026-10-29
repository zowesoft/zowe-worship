package serviceorder

import "context"

// Repository is the storage contract for services and their schedule items.
type Repository interface {
	Current(ctx context.Context) (Service, []Item, error)
	List(ctx context.Context) ([]Service, error)
	Create(ctx context.Context, name string) (Service, error)
	Use(ctx context.Context, serviceID string) error
	Rename(ctx context.Context, serviceID, name string) error
	AddScripture(ctx context.Context, serviceID string, input ScriptureInput) (Item, error)
	AddSong(ctx context.Context, serviceID string, input SongInput) (Item, error)
	RemoveItem(ctx context.Context, itemID string) error
	Reorder(ctx context.Context, itemIDs []string) error
	Item(ctx context.Context, itemID string) (Item, error)
	Items(ctx context.Context, serviceID string) ([]Item, error)
}
