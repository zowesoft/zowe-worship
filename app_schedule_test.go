package main

import (
	"errors"
	"testing"

	"zowe-worship/internal/serviceorder"
)

func scriptureAt(bookID, chapter, verse int) ScriptureRequest {
	return ScriptureRequest{TranslationID: "kjv", BookID: bookID, Chapter: chapter, VerseStart: verse, VerseEnd: verse}
}

func TestScheduleReorderDrivesPresentation(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	order, err := app.AddScriptureToService(scriptureAt(43, 3, 16))
	if err != nil {
		t.Fatalf("add first item: %v", err)
	}
	if order.Service == nil {
		t.Fatal("adding an item did not create a service")
	}
	if order, err = app.AddScriptureToService(scriptureAt(43, 4, 24)); err != nil {
		t.Fatalf("add second item: %v", err)
	}
	if order, err = app.AddScriptureToService(scriptureAt(58, 11, 1)); err != nil {
		t.Fatalf("add third item: %v", err)
	}

	before := itemIDs(order.Items)
	if _, err := app.ReorderServiceItems([]string{before[2], before[0], before[1]}); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	after, err := app.GetServiceOrder()
	if err != nil {
		t.Fatalf("read order: %v", err)
	}
	if got := itemIDs(after.Items); !equalIDs(got, []string{before[2], before[0], before[1]}) {
		t.Fatalf("reordered items = %v, want %v", got, []string{before[2], before[0], before[1]})
	}
	for index, item := range after.Items {
		if item.Position != index+1 {
			t.Fatalf("item %d has position %d", index, item.Position)
		}
	}

	// Advancing the schedule has to follow the stored order, not the order the
	// verses were added in.
	if _, err := app.PreviewServiceItem(after.Items[0].ID); err != nil {
		t.Fatalf("preview first item: %v", err)
	}
	state, err := app.NextServiceItem()
	if err != nil {
		t.Fatalf("next item: %v", err)
	}
	if state.Preview == nil || state.Preview.ItemID != after.Items[1].ID {
		t.Fatalf("next preview = %+v, want item %s", state.Preview, after.Items[1].ID)
	}

	if _, err := app.ReorderServiceItems([]string{before[0]}); err == nil {
		t.Fatal("a partial order was accepted")
	} else if !errors.Is(err, serviceorder.ErrInvalidOrder) {
		t.Fatalf("partial reorder err = %v", err)
	}

	unchanged, err := app.GetServiceOrder()
	if err != nil {
		t.Fatalf("read order after rejection: %v", err)
	}
	if got := itemIDs(unchanged.Items); !equalIDs(got, []string{before[2], before[0], before[1]}) {
		t.Fatalf("rejected reorder changed items to %v", got)
	}
}

func itemIDs(items []ServiceItemView) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func equalIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
