package detection

import (
	"testing"
	"time"

	"zopresent/internal/asr"
	"zopresent/internal/bible"
	"zopresent/internal/scripture"
)

// newClockedQueue returns a queue whose clock the test advances by hand, so the
// repeat window is exercised without sleeping.
func newClockedQueue(t *testing.T) (*Queue, *time.Time) {
	t.Helper()

	queue := newTestQueue(t)
	clock := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	queue.now = func() time.Time { return clock }
	return queue, &clock
}

func chapterWideDetection(t *testing.T, bookName string, chapter int) asr.Detection {
	t.Helper()

	book, ok := bible.LookupBook(bookName)
	if !ok {
		t.Fatalf("lookup %q", bookName)
	}
	return asr.Detection{
		Reference: scripture.Reference{Book: book, Chapter: chapter},
		Raw:       bookName,
	}
}

func TestFinalizeSettlesLiveCandidateUnderSameID(t *testing.T) {
	queue, _ := newClockedQueue(t)
	if segment := queue.BeginSegment(); segment != 1 {
		t.Fatalf("first segment = %d, want 1", segment)
	}

	live, err := queue.MergeLive("Turn with me to Philippians chapter 4", []asr.Detection{
		chapterWideDetection(t, "Philippians", 4),
	}, "service-1")
	if err != nil {
		t.Fatalf("merge partial: %v", err)
	}
	if len(live) != 1 {
		t.Fatalf("partial produced %d candidates, want 1", len(live))
	}

	settled, err := queue.Finalize("Turn with me to Philippians chapter 4, verses 6 and 7",
		[]asr.Detection{detectionOf(t, "Philippians 4:6-7")}, "service-1")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if settled.Segment != 1 || settled.Finalized != 1 {
		t.Fatalf("finalize result = %+v, want segment 1 and one settled card", settled)
	}
	if queue.CurrentSegment() != 0 {
		t.Fatalf("segment still open after finalize: %d", queue.CurrentSegment())
	}

	pending := queue.List("service-1")
	if len(pending) != 1 {
		t.Fatalf("queue holds %d candidates, want 1", len(pending))
	}
	candidate := pending[0]
	if candidate.ID != live[0].ID {
		t.Error("finalizing changed the candidate id; the operator would see the card move")
	}
	if !candidate.Finalized || candidate.FinalizedAt.IsZero() {
		t.Errorf("candidate is not finalized: %+v", candidate)
	}
	if candidate.Version < 2 {
		t.Errorf("version = %d, want the refinement counted", candidate.Version)
	}
	if candidate.Segment != 1 {
		t.Errorf("segment = %d, want 1", candidate.Segment)
	}
	if candidate.Reference.VerseStart != 6 || candidate.Reference.VerseEnd != 7 {
		t.Errorf("range = %d-%d, want 6-7", candidate.Reference.VerseStart, candidate.Reference.VerseEnd)
	}
}

func TestNewSegmentDoesNotRewriteSettledCard(t *testing.T) {
	queue, _ := newClockedQueue(t)

	queue.BeginSegment()
	if _, err := queue.Finalize("Let us turn to John 3:16", []asr.Detection{detectionOf(t, "John 3:16")}, "service-1"); err != nil {
		t.Fatalf("first finalize: %v", err)
	}

	queue.BeginSegment()
	if _, err := queue.MergeLive("Now Romans 8:28", []asr.Detection{detectionOf(t, "Romans 8:28")}, "service-1"); err != nil {
		t.Fatalf("second partial: %v", err)
	}

	var john, romans *Candidate
	pending := queue.List("service-1")
	for i := range pending {
		switch pending[i].Reference.String() {
		case "John 3:16":
			john = &pending[i]
		case "Romans 8:28":
			romans = &pending[i]
		}
	}
	if john == nil || romans == nil {
		t.Fatalf("pending = %+v, want John and Romans", pending)
	}
	if john.Transcript != "Let us turn to John 3:16" {
		t.Errorf("settled transcript = %q, want the first segment's text", john.Transcript)
	}
	if !john.Finalized || romans.Finalized {
		t.Errorf("finalized flags = %v/%v, want John settled and Romans live", john.Finalized, romans.Finalized)
	}
	if romans.Segment != 2 || john.Segment != 1 {
		t.Errorf("segments = %d/%d, want 2 and 1", romans.Segment, john.Segment)
	}

	settled, err := queue.Finalize("Now Romans 8:28", []asr.Detection{detectionOf(t, "Romans 8:28")}, "service-1")
	if err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	if settled.Segment != 2 {
		t.Errorf("second segment = %d, want 2", settled.Segment)
	}
	for _, candidate := range queue.List("service-1") {
		if !candidate.Finalized {
			t.Errorf("candidate %s still live after its segment closed", candidate.Reference.String())
		}
	}
}

func TestRepeatWithinWindowFoldsIntoPendingCard(t *testing.T) {
	queue, clock := newClockedQueue(t)

	queue.BeginSegment()
	first, err := queue.Finalize("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")}, "service-1")
	if err != nil {
		t.Fatalf("first finalize: %v", err)
	}
	*clock = clock.Add(30 * time.Second)

	queue.BeginSegment()
	second, err := queue.Finalize("John 3:16 again", []asr.Detection{detectionOf(t, "John 3:16")}, "service-1")
	if err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	if second.Repeats != 1 {
		t.Errorf("repeats = %d, want 1 folded hearing", second.Repeats)
	}
	if first.Segment != 1 || second.Segment != 2 {
		t.Errorf("segments = %d/%d, want 1 and 2", first.Segment, second.Segment)
	}

	pending := queue.List("service-1")
	if len(pending) != 1 {
		t.Fatalf("queue holds %d candidates, want the repeat folded into one", len(pending))
	}
	if pending[0].Repeats != 1 {
		t.Errorf("candidate repeats = %d, want 1", pending[0].Repeats)
	}
	if pending[0].Version != 2 {
		t.Errorf("version = %d, want 2", pending[0].Version)
	}
	if pending[0].Segment != 1 {
		t.Errorf("segment = %d, want the card to keep its original segment", pending[0].Segment)
	}
	if !pending[0].LastHeardAt.Equal(*clock) {
		t.Errorf("last heard = %v, want %v", pending[0].LastHeardAt, *clock)
	}
}

func TestRepeatAfterWindowQueuesNewCard(t *testing.T) {
	queue, clock := newClockedQueue(t)

	queue.BeginSegment()
	if _, err := queue.Finalize("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")}, "service-1"); err != nil {
		t.Fatalf("first finalize: %v", err)
	}
	*clock = clock.Add(RepeatWindow + time.Second)

	queue.BeginSegment()
	settled, err := queue.Finalize("John 3:16", []asr.Detection{detectionOf(t, "John 3:16")}, "service-1")
	if err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	if settled.Repeats != 0 {
		t.Errorf("repeats = %d, want a new card outside the window", settled.Repeats)
	}
	if pending := queue.List("service-1"); len(pending) != 2 {
		t.Fatalf("queue holds %d candidates, want a new card", len(pending))
	}
}

func TestSermonReferencesStayDistinctAcrossSegments(t *testing.T) {
	queue, clock := newClockedQueue(t)

	// The sample sermon names these in order, a chunk apart. Several share
	// verse numbers across different books — Philippians 4:6-7 and 1 Peter 5:7
	// both touch verse 7, and Matthew 6:25-34 contains Romans 8:28's verse 28 —
	// which used to fold into a single card because overlap compared verses
	// without the book and chapter.
	refs := []string{
		"Philippians 4:6-7", "Matthew 6:25-34", "Mark 4:35-41", "Isaiah 41:10",
		"1 Peter 5:7", "Psalms 23:1", "Romans 8:28",
	}
	for _, ref := range refs {
		queue.BeginSegment()
		if _, err := queue.Finalize(ref, []asr.Detection{detectionOf(t, ref)}, "service-1"); err != nil {
			t.Fatalf("finalize %q: %v", ref, err)
		}
		*clock = clock.Add(20 * time.Second)
	}

	pending := queue.List("service-1")
	if len(pending) != len(refs) {
		got := make([]string, 0, len(pending))
		for _, c := range pending {
			got = append(got, c.Reference.String())
		}
		t.Fatalf("queue holds %d candidates (%v), want all %d distinct references", len(pending), got, len(refs))
	}
	for i, ref := range refs {
		if pending[i].Reference.String() != ref {
			t.Errorf("card %d = %q, want %q", i, pending[i].Reference.String(), ref)
		}
		if pending[i].Repeats != 0 {
			t.Errorf("card %d repeats = %d, want 0: a different book is not a repeat", i, pending[i].Repeats)
		}
	}
}

func TestRefinementInsideWindowDoesNotCountAsRepeat(t *testing.T) {
	queue, clock := newClockedQueue(t)

	queue.BeginSegment()
	if _, err := queue.Finalize("Philippians 4", []asr.Detection{
		chapterWideDetection(t, "Philippians", 4),
	}, "service-1"); err != nil {
		t.Fatalf("first finalize: %v", err)
	}
	*clock = clock.Add(20 * time.Second)

	queue.BeginSegment()
	settled, err := queue.Finalize("Philippians 4:6-7", []asr.Detection{detectionOf(t, "Philippians 4:6-7")}, "service-1")
	if err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	if settled.Repeats != 0 {
		t.Errorf("repeats = %d, want the refinement folded without counting as a repeat", settled.Repeats)
	}

	pending := queue.List("service-1")
	if len(pending) != 1 {
		t.Fatalf("queue holds %d candidates, want 1 refined card", len(pending))
	}
	if pending[0].Repeats != 0 {
		t.Errorf("candidate repeats = %d, want 0", pending[0].Repeats)
	}
	if pending[0].Reference.VerseStart != 6 || pending[0].Reference.VerseEnd != 7 {
		t.Errorf("range = %d-%d, want 6-7", pending[0].Reference.VerseStart, pending[0].Reference.VerseEnd)
	}
}
