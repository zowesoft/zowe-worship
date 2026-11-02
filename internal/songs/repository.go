package songs

import "context"

// Repository stores songs and their lyric sections. Lyrics are entered by the
// church, so nothing here is bundled with the application.
type Repository interface {
	Create(ctx context.Context, song NewSong) (SongWithSections, error)
	List(ctx context.Context) ([]SongWithSections, error)
	Get(ctx context.Context, songID string) (SongWithSections, error)
	Update(ctx context.Context, songID string, input SongUpdate) (SongWithSections, error)
	Delete(ctx context.Context, songID string) error
	Duplicate(ctx context.Context, songID, newTitle string) (SongWithSections, error)
	Search(ctx context.Context, query string, limit int) ([]SectionHit, error)
	Section(ctx context.Context, songID, sectionID string) (Song, Section, error)
	CreateArrangement(ctx context.Context, input NewArrangement) (Arrangement, error)
	UpdateArrangement(ctx context.Context, arrangementID string, input ArrangementUpdate) (Arrangement, error)
	DeleteArrangement(ctx context.Context, arrangementID string) error
	ListArrangements(ctx context.Context, songID string) ([]Arrangement, error)
	GetArrangement(ctx context.Context, arrangementID string) (Arrangement, error)
}

