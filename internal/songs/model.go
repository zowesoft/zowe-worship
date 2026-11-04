package songs

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrSongNotFound        = errors.New("song not found")
	ErrSectionNotFound     = errors.New("song section not found")
	ErrArrangementNotFound = errors.New("song arrangement not found")
	ErrInvalidSong         = errors.New("invalid song")
	ErrInvalidArrangement  = errors.New("invalid song arrangement")
)

type Song struct {
	ID         string
	Title      string
	Author     string
	Copyright  string
	CCLINumber string
	Source     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Section struct {
	ID       string
	SongID   string
	Position int
	Label    string
	Text     string
}

type ArrangementItem struct {
	ID            string
	ArrangementID string
	SectionID     string
	Position      int
	Label         string
	Text          string
}

type Arrangement struct {
	ID        string
	SongID    string
	Name      string
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Items     []ArrangementItem
}

type SongWithSections struct {
	Song
	Sections     []Section
	Arrangements []Arrangement
}

// SectionUpdate edits a song's sections. An ID that already belongs to the song
// keeps that section's identity, so a service item scheduled from it still
// resolves after the lyrics change; an empty ID adds a section.
type SectionUpdate struct {
	ID    string
	Label string
	Text  string
}

type SongUpdate struct {
	Title      string
	Author     string
	Copyright  string
	CCLINumber string
	Source     string
	Sections   []SectionUpdate
}

type NewSection struct {
	Label string
	Text  string
}

type NewSong struct {
	Title      string
	Author     string
	Copyright  string
	CCLINumber string
	Source     string
	Sections   []NewSection
}

type NewArrangement struct {
	SongID     string
	Name       string
	IsDefault  bool
	SectionIDs []string
}

type ArrangementUpdate struct {
	Name       string
	IsDefault  bool
	SectionIDs []string
}

type SectionHit struct {
	Song    Song
	Section Section
}

func (n NewSong) normalised() (NewSong, error) {
	result := NewSong{
		Title:      strings.TrimSpace(n.Title),
		Author:     strings.TrimSpace(n.Author),
		Copyright:  strings.TrimSpace(n.Copyright),
		CCLINumber: strings.TrimSpace(n.CCLINumber),
		Source:     strings.TrimSpace(n.Source),
		Sections:   make([]NewSection, 0, len(n.Sections)),
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

func (u SongUpdate) normalised() (SongUpdate, error) {
	result := SongUpdate{
		Title:      strings.TrimSpace(u.Title),
		Author:     strings.TrimSpace(u.Author),
		Copyright:  strings.TrimSpace(u.Copyright),
		CCLINumber: strings.TrimSpace(u.CCLINumber),
		Source:     strings.TrimSpace(u.Source),
		Sections:   make([]SectionUpdate, 0, len(u.Sections)),
	}

	if result.Title == "" {
		return SongUpdate{}, fmt.Errorf("%w: a song title is required", ErrInvalidSong)
	}
	seen := make(map[string]bool, len(u.Sections))
	for _, section := range u.Sections {
		label := strings.TrimSpace(section.Label)
		text := strings.TrimSpace(section.Text)
		if label == "" || text == "" {
			return SongUpdate{}, fmt.Errorf("%w: every section needs a label and lyrics", ErrInvalidSong)
		}
		if section.ID != "" {
			if seen[section.ID] {
				return SongUpdate{}, fmt.Errorf("%w: the same section was sent twice", ErrInvalidSong)
			}
			seen[section.ID] = true
		}
		result.Sections = append(result.Sections, SectionUpdate{ID: section.ID, Label: label, Text: text})
	}
	if len(result.Sections) == 0 {
		return SongUpdate{}, fmt.Errorf("%w: add at least one section", ErrInvalidSong)
	}
	return result, nil
}

func (a NewArrangement) normalised() (NewArrangement, error) {
	result := NewArrangement{
		SongID:     strings.TrimSpace(a.SongID),
		Name:       strings.TrimSpace(a.Name),
		IsDefault:  a.IsDefault,
		SectionIDs: make([]string, 0, len(a.SectionIDs)),
	}
	if result.SongID == "" {
		return NewArrangement{}, fmt.Errorf("%w: song id is required", ErrInvalidArrangement)
	}
	if result.Name == "" {
		return NewArrangement{}, fmt.Errorf("%w: arrangement name is required", ErrInvalidArrangement)
	}
	for _, id := range a.SectionIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			result.SectionIDs = append(result.SectionIDs, trimmed)
		}
	}
	if len(result.SectionIDs) == 0 {
		return NewArrangement{}, fmt.Errorf("%w: arrangement must have at least one section", ErrInvalidArrangement)
	}
	return result, nil
}

func (u ArrangementUpdate) normalised() (ArrangementUpdate, error) {
	result := ArrangementUpdate{
		Name:       strings.TrimSpace(u.Name),
		IsDefault:  u.IsDefault,
		SectionIDs: make([]string, 0, len(u.SectionIDs)),
	}
	if result.Name == "" {
		return ArrangementUpdate{}, fmt.Errorf("%w: arrangement name is required", ErrInvalidArrangement)
	}
	for _, id := range u.SectionIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			result.SectionIDs = append(result.SectionIDs, trimmed)
		}
	}
	if len(result.SectionIDs) == 0 {
		return ArrangementUpdate{}, fmt.Errorf("%w: arrangement must have at least one section", ErrInvalidArrangement)
	}
	return result, nil
}
