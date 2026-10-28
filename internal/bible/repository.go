package bible

import "context"

// Repository is the storage contract the rest of the application depends on.
// The SQLite implementation can be replaced without touching UI or services.
type Repository interface {
	ListTranslations(ctx context.Context) ([]Translation, error)
	Translation(ctx context.Context, id string) (Translation, error)
	Install(ctx context.Context, data TranslationData) (Translation, error)
	Remove(ctx context.Context, id string) error
	Search(ctx context.Context, req SearchRequest) ([]SearchResult, error)
	// Passage returns a chapter, or a verse range inside it, in verse order.
	// startVerse and endVerse are zero when the whole chapter is requested.
	Passage(ctx context.Context, translationID string, bookID, chapter, startVerse, endVerse int) ([]SearchResult, error)
}
