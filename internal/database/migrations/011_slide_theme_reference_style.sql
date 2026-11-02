-- The reference label is the one part of a slide an operator wants smaller,
-- quieter and sometimes in a different voice than the passage itself, so it
-- carries its own style instead of borrowing every property from the body text.
--
-- An empty font, colour or alignment means "same as the slide text", and a size
-- of zero means the same. That keeps a theme that only wants a different colour
-- from having to restate the whole style.
ALTER TABLE slide_templates ADD COLUMN text_case TEXT NOT NULL DEFAULT 'none'
    CHECK (text_case IN ('none', 'capitalize', 'uppercase', 'lowercase', 'small-caps'));

ALTER TABLE slide_templates ADD COLUMN ref_font_family TEXT NOT NULL DEFAULT '';

ALTER TABLE slide_templates ADD COLUMN ref_font_size REAL NOT NULL DEFAULT 0;

ALTER TABLE slide_templates ADD COLUMN ref_color TEXT NOT NULL DEFAULT ''
    CHECK (ref_color = '' OR ref_color GLOB '#[0-9a-fA-F]*');

ALTER TABLE slide_templates ADD COLUMN ref_align TEXT NOT NULL DEFAULT ''
    CHECK (ref_align IN ('', 'left', 'center', 'right'));

ALTER TABLE slide_templates ADD COLUMN ref_case TEXT NOT NULL DEFAULT 'none'
    CHECK (ref_case IN ('none', 'capitalize', 'uppercase', 'lowercase', 'small-caps'));

ALTER TABLE slide_templates ADD COLUMN ref_bold INTEGER NOT NULL DEFAULT 0;

ALTER TABLE slide_templates ADD COLUMN ref_italic INTEGER NOT NULL DEFAULT 0;
