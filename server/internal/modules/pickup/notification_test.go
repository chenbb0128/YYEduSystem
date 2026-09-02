package pickup

import (
	"context"
	"testing"
	"time"
)

func TestMemoryNotificationOutboxLifecycle(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	notification, err := store.CreateNotification(ctx, 1, CreateNotificationParams{StudentID: 7, Kind: "pickup_status", Title: "孩子已到班", Content: "已完成到班核对"})
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.ListNotificationOutbox(ctx, time.Now().UTC().Add(time.Second), time.Now().UTC().Add(-time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].NotificationID != notification.ID || events[0].Status != "pending" {
		t.Fatalf("outbox = %+v", events)
	}
	eventID := events[0].ID

	claimed, err := store.ClaimNotificationOutbox(ctx, 1, eventID, time.Now().UTC())
	if err != nil || !claimed {
		t.Fatalf("claim = %v, %v", claimed, err)
	}
	next := time.Now().UTC().Add(time.Minute)
	if err := store.CompleteNotificationOutbox(ctx, 1, eventID, "pending", &next, "微信暂时不可用"); err != nil {
		t.Fatal(err)
	}
	events, err = store.ListNotificationOutbox(ctx, next.Add(-time.Second), time.Now().UTC().Add(-time.Minute), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("future retry should not be ready: %+v", events)
	}

	if err := store.RetryNotification(ctx, 1, notification.ID); err == nil {
		t.Fatal("retry should require a failed outbox")
	}
	claimed, err = store.ClaimNotificationOutbox(ctx, 1, eventID, time.Now().UTC())
	if err != nil || !claimed {
		t.Fatalf("reclaim = %v, %v", claimed, err)
	}
	if err := store.CompleteNotificationOutbox(ctx, 1, eventID, "failed", &next, "最终失败"); err != nil {
		t.Fatal(err)
	}
	if err := store.RetryNotification(ctx, 1, notification.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := store.FindNotification(ctx, 1, notification.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "pending" || updated.DeliveryAttempts != 0 {
		t.Fatalf("retried notification = %+v", updated)
	}
}

func TestMemoryDeliveryLogIsIdempotent(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	first, err := store.CreateNotificationDeliveryLog(ctx, 1, CreateDeliveryLogParams{NotificationID: 11, ParentAccountID: 21, MessageKind: "pickup", TemplateID: "tmpl"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateNotificationDeliveryLog(ctx, 1, CreateDeliveryLogParams{NotificationID: 11, ParentAccountID: 21, MessageKind: "pickup", TemplateID: "tmpl"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate delivery log IDs = %d and %d", first.ID, second.ID)
	}
	if err := store.SetNotificationDeliveryLogStatus(ctx, 1, SetDeliveryLogStatusParams{ID: first.ID, Status: "sent", Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	logs, err := store.ListNotificationDeliveryLogs(ctx, 1, nil, "sent")
	if err != nil || len(logs) != 1 {
		t.Fatalf("logs = %+v, err = %v", logs, err)
	}
}
