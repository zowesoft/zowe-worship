-- Reusable themes for projected slides. A theme only decides how a slide looks:
-- the scripture and lyric text itself is never copied or rewritten here.
--
-- Sizes are percentages of the output height rather than pixels, so the same
-- theme reads correctly on a laptop preview pane and on a 4K projector.
CREATE TABLE slide_templates (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('scripture', 'lyrics', 'text')),
    is_default  INTEGER NOT NULL DEFAULT 0,

    font_family TEXT NOT NULL,
    font_size   REAL NOT NULL,
    line_height REAL NOT NULL,
    bold        INTEGER NOT NULL DEFAULT 0,
    italic      INTEGER NOT NULL DEFAULT 0,
    align       TEXT NOT NULL CHECK (align IN ('left', 'center', 'right')),
    color       TEXT NOT NULL,
    shadow      INTEGER NOT NULL DEFAULT 0,
    margin      REAL NOT NULL,

    background_kind   TEXT NOT NULL CHECK (background_kind IN ('solid', 'gradient', 'image')),
    background_color  TEXT NOT NULL DEFAULT '',
    background_color2 TEXT NOT NULL DEFAULT '',
    background_angle  INTEGER NOT NULL DEFAULT 180,
    background_media  TEXT NOT NULL DEFAULT '',

    reference_placement TEXT NOT NULL CHECK (reference_placement IN ('none', 'top', 'bottom')),

    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- Exactly one default per kind, enforced where it is read rather than by a
-- separate settings row that could disagree with these flags.
CREATE UNIQUE INDEX slide_templates_one_default ON slide_templates (kind) WHERE is_default = 1;

CREATE INDEX slide_templates_kind ON slide_templates (kind, name COLLATE NOCASE);
