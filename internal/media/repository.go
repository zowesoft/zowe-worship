package media

import "context"

// Repository is the catalog of media the library owns. The files themselves are
// handled by Library, so the database never has to know where they live.
type Repository interface {
	List(ctx context.Context) ([]Asset, error)
	Add(ctx context.Context, asset Asset) error
	Remove(ctx context.Context, id string) error
	ByID(ctx context.Context, id string) (Asset, error)
	ByChecksum(ctx context.Context, checksum string) (Asset, bool, error)
	SavePresentation(ctx context.Context, id string, presentation Presentation) error
	SetRestored(ctx context.Context, asset Asset) error
}
