package songs

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrSongNotFound    = errors.New("song not found")
	ErrSectionNotFound = errors.New("song section not found")
	ErrInvalidSong     = errors.New("invalid song")
)

type Song struct {
	ID        string
	Title     string
	Author    string
	Copyright string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Section struct {
	ID       string
	SongID   string
	Position int
	Label    string
	Text     string
}

type SongWithSections struct {
	Song
	Sections []Section
}

type NewSection struct {
	Label string
	Text  string
}

type NewSong struct {
	Title     string
	Author    string
	Copyright string
	Sections  []NewSection
}

type SectionHit struct {
	Song    Song
	Section Section
}

func (n NewSong) normalised() (NewSong, error) {
	result := NewSong{
		Title:     strings.TrimSpace(n.Title),
		Author:    strings.TrimSpace(n.Author),
		Copyright: strings.TrimSpace(n.Copyright),
		Sections:  make([]NewSection, 0, len(n.Sections)),
	}

	if result.Title == "" {
		return NewSong{}, fmt.Errorf("%w: a song title is required", ErrInvalidSong)
	}
	for _, section := range n.Sections {
		label := strings.TrimSpace(section.Label)
		text := strings.TrimSpace(section.Text)
		if label == "" || text == "" {
			return NewSong{}, fmt.Errorf("%w: every section needs a label and lyrics", ErrInvalidSong)
		}
		result.Sections = append(result.Sections, NewSection{Label: label, Text: text})
	}
	if len(result.Sections) == 0 {
		return NewSong{}, fmt.Errorf("%w: add at least one section", ErrInvalidSong)
	}
	return result, nil
}
