package themes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"zowe-worship/internal/database"
)

const themeColumns = `id, name, kind, is_default, font_family, font_size, line_height, bold, italic,
	align, color, shadow, margin, text_case, background_kind, background_color, background_color2,
	background_angle, background_media, reference_placement, ref_font_family, ref_font_size, ref_color,
	ref_align, ref_case, ref_bold, ref_italic, created_at, updated_at`

type SQLiteRepository struct {
	db *sql.DB
}

var _ Repository = (*SQLiteRepository)(nil)

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List(ctx context.Context) ([]Theme, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+themeColumns+` FROM slide_templates
		ORDER BY kind, is_default DESC, name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list themes: %w", err)
	}
	defer rows.Close()

	themes := make([]Theme, 0)
	for rows.Next() {
		theme, err := scanTheme(rows)
		if err != nil {
			return nil, fmt.Errorf("list themes: %w", err)
		}
		themes = append(themes, theme.Normalised())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list themes: %w", err)
	}
	return themes, nil
}

func (r *SQLiteRepository) Get(ctx context.Context, id string) (Theme, error) {
	theme, err := scanTheme(r.db.QueryRowContext(ctx,
		`SELECT `+themeColumns+` FROM slide_templates WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Theme{}, ErrThemeNotFound
	}
	if err != nil {
		return Theme{}, fmt.Errorf("read theme: %w", err)
	}
	return theme.Normalised(), nil
}

// Create stores a new theme. The first theme for a kind becomes that kind's
// default, because a church with one scripture theme plainly means it for every
// scripture slide.
func (r *SQLiteRepository) Create(ctx context.Context, theme Theme) (Theme, error) {
	if err := theme.Validate(); err != nil {
		return Theme{}, err
	}

	var taken int
	if err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM slide_templates WHERE kind = ? AND name = ? COLLATE NOCASE`,
		theme.Kind, theme.Name).Scan(&taken); err == nil {
		return Theme{}, ErrNameTaken
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Theme{}, fmt.Errorf("check theme name: %w", err)
	}

	var siblings int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM slide_templates WHERE kind = ?`, theme.Kind).Scan(&siblings); err != nil {
		return Theme{}, fmt.Errorf("count themes of kind: %w", err)
	}
	theme.IsDefault = siblings == 0

	id, err := database.NewID()
	if err != nil {
		return Theme{}, err
	}
	theme.ID = id
	now := time.Now().UTC().Truncate(time.Second)
	theme.CreatedAt, theme.UpdatedAt = now, now

	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO slide_templates (`+themeColumns+`) VALUES (`+placeholders(29)+`)`,
		themeArgs(theme)...); err != nil {
		return Theme{}, fmt.Errorf("create theme: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *SQLiteRepository) Update(ctx context.Context, theme Theme) (Theme, error) {
	if err := theme.Validate(); err != nil {
		return Theme{}, err
	}

	var taken int
	if err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM slide_templates WHERE kind = ? AND name = ? COLLATE NOCASE AND id <> ?`,
		theme.Kind, theme.Name, theme.ID).Scan(&taken); err == nil {
		return Theme{}, ErrNameTaken
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Theme{}, fmt.Errorf("check theme name: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE slide_templates
		SET name = ?, kind = ?, font_family = ?, font_size = ?, line_height = ?, bold = ?,
		    italic = ?, align = ?, color = ?, shadow = ?, margin = ?, text_case = ?,
		    background_kind = ?, background_color = ?, background_color2 = ?, background_angle = ?,
		    background_media = ?, reference_placement = ?, ref_font_family = ?, ref_font_size = ?,
		    ref_color = ?, ref_align = ?, ref_case = ?, ref_bold = ?, ref_italic = ?, updated_at = ?
		WHERE id = ?`,
		theme.Name, theme.Kind, theme.Style.FontFamily, theme.Style.FontSize, theme.Style.LineHeight,
		boolToInt(theme.Style.Bold), theme.Style.Italic, theme.Style.Align, theme.Style.Color,
		boolToInt(theme.Style.Shadow), theme.Style.Margin, theme.Style.Case,
		theme.Background.Kind, theme.Background.Color, theme.Background.Color2,
		theme.Background.Angle, theme.Background.MediaID, theme.Reference.Placement,
		theme.Reference.FontFamily, theme.Reference.FontSize, theme.Reference.Color,
		theme.Reference.Align, theme.Reference.Case, boolToInt(theme.Reference.Bold),
		boolToInt(theme.Reference.Italic),
		time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), theme.ID,
	)
	if err != nil {
		return Theme{}, fmt.Errorf("update theme: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Theme{}, fmt.Errorf("update theme: %w", err)
	}
	if changed == 0 {
		return Theme{}, ErrThemeNotFound
	}
	return r.Get(ctx, theme.ID)
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM slide_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete theme: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete theme: %w", err)
	}
	if deleted == 0 {
		return ErrThemeNotFound
	}
	return nil
}

// MakeDefault moves the default flag in one transaction, so the partial unique
// index can never see two themes claiming the same kind.
func (r *SQLiteRepository) MakeDefault(ctx context.Context, id string) error {
	theme, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin default change: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE slide_templates SET is_default = 0 WHERE kind = ?`, theme.Kind); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear previous default: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE slide_templates SET is_default = 1 WHERE id = ?`, id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("set default theme: %w", err)
	}
	return tx.Commit()
}

// Default reports the theme a kind projects with when nothing else is chosen.
func (r *SQLiteRepository) Default(ctx context.Context, kind Kind) (Theme, error) {
	theme, err := scanTheme(r.db.QueryRowContext(ctx,
		`SELECT `+themeColumns+` FROM slide_templates WHERE kind = ? AND is_default = 1`, kind))
	if errors.Is(err, sql.ErrNoRows) {
		return Theme{}, ErrThemeNotFound
	}
	if err != nil {
		return Theme{}, fmt.Errorf("read default theme: %w", err)
	}
	return theme.Normalised(), nil
}

func themeArgs(theme Theme) []any {
	return []any{
		theme.ID, theme.Name, theme.Kind, boolToInt(theme.IsDefault),
		theme.Style.FontFamily, theme.Style.FontSize, theme.Style.LineHeight,
		boolToInt(theme.Style.Bold), boolToInt(theme.Style.Italic), theme.Style.Align,
		theme.Style.Color, boolToInt(theme.Style.Shadow), theme.Style.Margin, theme.Style.Case,
		theme.Background.Kind, theme.Background.Color, theme.Background.Color2,
		theme.Background.Angle, theme.Background.MediaID, theme.Reference.Placement,
		theme.Reference.FontFamily, theme.Reference.FontSize, theme.Reference.Color,
		theme.Reference.Align, theme.Reference.Case, boolToInt(theme.Reference.Bold),
		boolToInt(theme.Reference.Italic),
		theme.CreatedAt.Format(time.RFC3339), theme.UpdatedAt.Format(time.RFC3339),
	}
}

func scanTheme(row interface{ Scan(...any) error }) (Theme, error) {
	var (
		theme          Theme
		isDefault      int
		bold           int
		italic         int
		shadow         int
		textCase       string
		refCase        string
		refBold        int
		refItalic      int
		createdAt      string
		updatedAt      string
		backgroundKind string
	)
	err := row.Scan(
		&theme.ID, &theme.Name, &theme.Kind, &isDefault,
		&theme.Style.FontFamily, &theme.Style.FontSize, &theme.Style.LineHeight,
		&bold, &italic, &theme.Style.Align, &theme.Style.Color, &shadow, &theme.Style.Margin,
		&textCase, &backgroundKind, &theme.Background.Color, &theme.Background.Color2,
		&theme.Background.Angle, &theme.Background.MediaID, &theme.Reference.Placement,
		&theme.Reference.FontFamily, &theme.Reference.FontSize, &theme.Reference.Color,
		&theme.Reference.Align, &refCase, &refBold, &refItalic,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return Theme{}, err
	}

	theme.IsDefault, theme.Style.Bold, theme.Style.Italic, theme.Style.Shadow = isDefault != 0, bold != 0, italic != 0, shadow != 0
	theme.Style.Case = TextCase(textCase)
	theme.Reference.Case = TextCase(refCase)
	theme.Reference.Bold, theme.Reference.Italic = refBold != 0, refItalic != 0
	theme.Background.Kind = BackgroundKind(backgroundKind)
	theme.CreatedAt, err = parseTime(createdAt, "creation")
	if err != nil {
		return Theme{}, err
	}
	theme.UpdatedAt, err = parseTime(updatedAt, "update")
	if err != nil {
		return Theme{}, err
	}
	return theme, nil
}

func parseTime(value, what string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse theme %s time: %w", what, err)
	}
	return parsed, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func placeholders(count int) string {
	out := ""
	for index := 0; index < count; index++ {
		if index > 0 {
			out += ", "
		}
		out += "?"
	}
	return out
}
