package detection

import (
	"testing"

	"zopresent/internal/asr"
	"zopresent/internal/bible"
	"zopresent/internal/scripture"
)

func romansEight(t *testing.T) scripture.Reference {
	t.Helper()
	book, ok := bible.LookupBook("romans")
	if !ok {
		t.Fatal("no romans book in canon")
	}
	return scripture.Reference{Book: book, Chapter: 8, VerseStart: 28, VerseEnd: 28}
}

func allVersesExist(int) bool { return true }

func TestContextualHintResolvesAgainstTheEstablishedPassage(t *testing.T) {
	established := romansEight(t)
	got := DetectContextualReferences("now read verse twenty-nine", established, nil, allVersesExist)
	if len(got) != 1 {
		t.Fatalf("hint count = %d, want 1 (%+v)", len(got), got)
	}
	ref := got[0].Reference
	if ref.Book.Name != "Romans" || ref.Chapter != 8 || ref.VerseStart != 29 || ref.VerseEnd != 29 {
		t.Fatalf("reference = %s, want Romans 8:29", ref.String())
	}
	if got[0].Method != MethodContextualReference {
		t.Fatalf("method = %q, want %q", got[0].Method, MethodContextualReference)
	}
}

func TestContextualHintRequiresAnEstablishedPassage(t *testing.T) {
	// Without a passage there is nothing to resolve the mention against.
	if got := DetectContextualReferences("verse 29", scripture.Reference{}, nil, allVersesExist); len(got) != 0 {
		t.Fatalf("hints without context = %+v, want none", got)
	}
}

func TestContextualHintDropsNonExistentVerse(t *testing.T) {
	established := romansEight(t)
	got := DetectContextualReferences("verse one hundred", established, nil, func(verse int) bool {
		return verse <= 39
	})
	if len(got) != 0 {
		t.Fatalf("impossible verse produced %+v, want none", got)
	}
}

func TestContextualHintSkipsWhatAnExplicitReferenceCovers(t *testing.T) {
	established := romansEight(t)
	explicit := []asr.Detection{{Reference: romansEight(t), Raw: "Romans 8:28"}}
	got := DetectContextualReferences("read verse twenty-eight", established, explicit, allVersesExist)
	if len(got) != 0 {
		t.Fatalf("explicit-covered verse produced %+v, want none", got)
	}
	// A bare mention outside the explicit range still resolves.
	got = DetectContextualReferences("read verse twenty-nine", established, explicit, allVersesExist)
	if len(got) != 1 || got[0].Reference.VerseStart != 29 {
		t.Fatalf("hint outside explicit range = %+v, want Romans 8:29", got)
	}
}

func TestContextualHintSkipsWholeChapterExplicit(t *testing.T) {
	established := romansEight(t)
	explicit := []asr.Detection{{Reference: scripture.Reference{Book: established.Book, Chapter: established.Chapter}, Raw: "Romans 8"}}
	got := DetectContextualReferences("read verse twenty-nine", established, explicit, allVersesExist)
	if len(got) != 0 {
		t.Fatalf("explicit whole-chapter reference produced %+v, want none", got)
	}
}

func TestContextualHintSkipsOtherChapter(t *testing.T) {
	established := romansEight(t)
	other, ok := bible.LookupBook("john")
	if !ok {
		t.Fatal("no john book in canon")
	}
	explicit := []asr.Detection{{Reference: scripture.Reference{Book: other, Chapter: 3, VerseStart: 29, VerseEnd: 29}, Raw: "John 3:29"}}
	got := DetectContextualReferences("read verse twenty-nine", established, explicit, allVersesExist)
	if len(got) != 1 || got[0].Reference.VerseStart != 29 || got[0].Reference.Book.Name != "Romans" {
		t.Fatalf("hint in another book produced %+v, want Romans 8:29", got)
	}
}

func TestContextualHintDedupesAndCaps(t *testing.T) {
	established := romansEight(t)
	got := DetectContextualReferences("verse 16 verse 16", established, nil, allVersesExist)
	if len(got) != 1 {
		t.Fatalf("dedupe: got %d candidates, want 1", len(got))
	}
	got = DetectContextualReferences("verse 1, verse 2, verse 3, verse 4, verse 5", established, nil, allVersesExist)
	if len(got) != maxContextualPerSegment {
		t.Fatalf("cap: got %d candidates, want %d", len(got), maxContextualPerSegment)
	}
}

func TestContextualHintEstablishesChapterOnlyMatches(t *testing.T) {
	// A passage established on one verse still resolves a verse anywhere in
	// the chapter, including the passage's own verses.
	established := romansEight(t)
	got := DetectContextualReferences("verse 28", established, nil, allVersesExist)
	if len(got) != 1 || got[0].Reference.VerseStart != 28 {
		t.Fatalf("hint for the established verse = %+v, want Romans 8:28", got)
	}
}
