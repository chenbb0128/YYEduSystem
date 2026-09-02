package summary

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryStoreDoesNotRegeneratePublishedSummary(t *testing.T) {
	store := NewMemoryStore()
	date := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)
	item, err := store.Generate(context.Background(), 1, GenerateParams{
		SummaryDate: date,
		Content:     "初稿",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetStatus(context.Background(), 1, item.ID, StatusPublished); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Generate(context.Background(), 1, GenerateParams{
		SummaryDate: date,
		Content:     "不应覆盖已发布内容",
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("regenerate published summary error = %v, want %v", err, ErrInvalidState)
	}
	current, err := store.Find(context.Background(), 1, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Content != "初稿" || current.Status != StatusPublished {
		t.Fatalf("published summary changed = %+v", current)
	}
}

func TestMemoryStoreTracksVersionsAndReadState(t *testing.T) {
	store := NewMemoryStore()
	date := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)
	item, err := store.Generate(context.Background(), 1, GenerateParams{SummaryDate: date, Content: "初稿"})
	if err != nil {
		t.Fatal(err)
	}
	item, err = store.Update(context.Background(), 1, UpdateParams{ID: item.ID, Content: "保存后的草稿"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Version != 2 {
		t.Fatalf("draft version = %d, want 2", item.Version)
	}
	if _, err := store.SetStatus(context.Background(), 1, item.ID, StatusPublished); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkRead(context.Background(), 1, item.ID, 9, item.Version); err != nil {
		t.Fatal(err)
	}
	if readAt, err := store.ReadAt(context.Background(), 1, item.ID, 9); err != nil || readAt == nil {
		t.Fatalf("read state = %v, err = %v", readAt, err)
	}
	item, err = store.Correct(context.Background(), 1, CorrectParams{ID: item.ID, Content: "更正后的总结", Reason: "补充现场情况"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Version != 3 || item.Status != StatusPublished {
		t.Fatalf("corrected item = %+v", item)
	}
	if readAt, err := store.ReadAt(context.Background(), 1, item.ID, 9); err != nil || readAt != nil {
		t.Fatalf("read state after correction = %v, err = %v, want unread", readAt, err)
	}
	versions, err := store.ListVersions(context.Background(), 1, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 4 {
		t.Fatalf("versions = %d, want 4", len(versions))
	}
	if _, err := store.Withdraw(context.Background(), 1, item.ID, "需要重新核对"); err != nil {
		t.Fatal(err)
	}
	if current, err := store.Find(context.Background(), 1, item.ID); err != nil || current.Status != StatusWithdrawn {
		t.Fatalf("withdrawn item = %+v, err = %v", current, err)
	}
}
