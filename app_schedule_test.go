package main

import (
	"errors"
	"strings"
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

func TestServiceSwitchingThroughBindings(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	first, err := app.CreateService("Christmas Eve")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if _, err := app.AddScriptureToService(scriptureAt(43, 3, 16)); err != nil {
		t.Fatalf("add item: %v", err)
	}

	second, err := app.CreateService("Easter")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if second.Service == nil || second.Service.Name != "Easter" {
		t.Fatalf("current after create = %+v", second.Service)
	}
	if len(second.Items) != 0 {
		t.Fatalf("new service inherited items: %+v", second.Items)
	}

	back, err := app.UseService(first.Service.ID)
	if err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if back.Service == nil || back.Service.Name != "Christmas Eve" || len(back.Items) != 1 {
		t.Fatalf("switched view = %+v", back)
	}

	renamed, err := app.RenameService(first.Service.ID, "  Baptism Service  ")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Service == nil || renamed.Service.Name != "Baptism Service" {
		t.Fatalf("renamed view = %+v", renamed.Service)
	}

	list, err := app.ListServices()
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("services = %+v, want two", list)
	}

	if _, err := app.UseService("missing"); err == nil {
		t.Fatal("switching to a missing service succeeded")
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

func TestRestoreProjectionPutsTheScheduledItemBack(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	order, err := app.AddScriptureToService(scriptureAt(43, 3, 16))
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	itemID := order.Items[0].ID

	if _, err := app.PreviewServiceItem(itemID); err != nil {
		t.Fatalf("preview item: %v", err)
	}
	projected, err := app.SendPreviewLive()
	if err != nil {
		t.Fatalf("send live: %v", err)
	}
	if projected.Live == nil || projected.Live.Reference != "John 3:16" {
		t.Fatalf("live after project = %+v", projected.Live)
	}

	// Take it down, then bring it back from the history panel.
	if _, err := app.ClearLive(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("state after clear: %v", err)
	}
	if state.Live != nil {
		t.Fatalf("clear did not black the screen: %+v", state.Live)
	}

	entries, err := app.RecentProjections()
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	// Newest first: the clear row then the projection row. The clear is
	// not restorable; the scheduled projection is.
	if len(entries) != 2 {
		t.Fatalf("history rows = %d, want 2", len(entries))
	}
	if entries[0].Restorable {
		t.Fatalf("a clear row was marked restorable: %+v", entries[0])
	}
	restorable := entries[1]
	if !restorable.Restorable {
		t.Fatalf("a scheduled projection was not marked restorable: %+v", restorable)
	}

	restored, err := app.RestoreProjection(restorable.ID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Live == nil || restored.Live.Reference != "John 3:16" || restored.Live.ItemID != itemID {
		t.Fatalf("live after restore = %+v", restored.Live)
	}

	// The restore itself is recorded, so the panel shows it too.
	entries, err = app.RecentProjections()
	if err != nil {
		t.Fatalf("history after restore: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("history rows after restore = %d, want 3", len(entries))
	}
	if entries[0].Title != restorable.Title {
		t.Fatalf("restored row title = %q, want %q", entries[0].Title, restorable.Title)
	}
}

func TestRestoreProjectionRefusesAdHocAndUnknownEntries(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	// Ad hoc scripture projected straight from the Bible tab carries no
	// schedule item, so there is nothing to restore.
	if _, err := app.ProjectScripture(scriptureAt(19, 23, 1)); err != nil {
		t.Fatalf("project ad hoc: %v", err)
	}
	entries, err := app.RecentProjections()
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 1 || entries[0].Restorable {
		t.Fatalf("adhoc rows = %+v", entries)
	}
	if _, err := app.RestoreProjection(entries[0].ID); err == nil || !strings.Contains(err.Error(), "ad hoc") {
		t.Fatalf("ad hoc restore err = %v, want a refusal", err)
	}

	if _, err := app.RestoreProjection("missing-id"); err == nil || !strings.Contains(err.Error(), "no longer in the history") {
		t.Fatalf("unknown entry err = %v, want a not-found refusal", err)
	}
}
