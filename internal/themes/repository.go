package themes

import "context"

// Repository stores reusable theme definitions. It owns the one-default-per-kind
// rule, because two themes both claiming to be the scripture default is a
// question nothing in the UI can answer.
type Repository interface {
	List(ctx context.Context) ([]Theme, error)
	Get(ctx context.Context, id string) (Theme, error)
	Create(ctx context.Context, theme Theme) (Theme, error)
	Update(ctx context.Context, theme Theme) (Theme, error)
	Delete(ctx context.Context, id string) error
	MakeDefault(ctx context.Context, id string) error
	Default(ctx context.Context, kind Kind) (Theme, error)
}
