package serviceorder

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNoService       = errors.New("no service exists")
	ErrServiceNotFound = errors.New("service not found")
	ErrItemNotFound    = errors.New("service item not found")
	ErrInvalidService  = errors.New("invalid service")
	ErrInvalidOrder    = errors.New("invalid schedule order")
)

type ItemKind string

const (
	KindScripture ItemKind = "scripture"
	KindSong      ItemKind = "song"
)

type Service struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Item struct {
	ID            string
	ServiceID     string
	Position      int
	Kind          ItemKind
	Title         string
	TranslationID string
	BookID        int
	Chapter       int
	VerseStart    int
	VerseEnd      int
	SongID        string
	SectionID     string
	CreatedAt     time.Time
}

type ScriptureInput struct {
	Title         string
	TranslationID string
	BookID        int
	Chapter       int
	VerseStart    int
	VerseEnd      int
}

type SongInput struct {
	Title     string
	SongID    string
	SectionID string
}

func (s ScriptureInput) validate() error {
	if strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.TranslationID) == "" {
		return ErrInvalidService
	}
	if s.BookID <= 0 || s.Chapter <= 0 || s.VerseStart <= 0 {
		return ErrInvalidService
	}
	if s.VerseEnd == 0 {
		return ErrInvalidService
	}
	if s.VerseEnd < s.VerseStart {
		return ErrInvalidService
	}
	return nil
}

func (s SongInput) validate() error {
	if strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.SongID) == "" || strings.TrimSpace(s.SectionID) == "" {
		return ErrInvalidService
	}
	return nil
}
