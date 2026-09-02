package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/wechat"
)

const notificationDeliveryTimeout = 8 * time.Second

// NotificationDispatcher consumes durable outbox rows. It is deliberately
// independent from the HTTP request lifecycle: a WeChat outage only changes
// delivery state and never rolls back the business operation that created the
// notification.
type NotificationDispatcher struct {
	store       pickup.Store
	parents     parent.Store
	sender      *wechat.Client
	cfg         config.WeChatConfig
	logger      *slog.Logger
	interval    time.Duration
	lease       time.Duration
	maxAttempts int
}

func NewNotificationDispatcher(store pickup.Store, parents parent.Store, sender *wechat.Client, cfg config.WeChatConfig, logger *slog.Logger, interval, lease time.Duration, maxAttempts int) *NotificationDispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &NotificationDispatcher{store: store, parents: parents, sender: sender, cfg: cfg, logger: logger, interval: interval, lease: lease, maxAttempts: maxAttempts}
}

func (d *NotificationDispatcher) Run(ctx context.Context) {
	if d == nil || d.store == nil || d.parents == nil || d.sender == nil || !d.cfg.HasSubscribeTemplates() {
		return
	}
	d.dispatchBatch(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.dispatchBatch(ctx)
		}
	}
}

func (d *NotificationDispatcher) dispatchBatch(ctx context.Context) {
	now := time.Now().UTC()
	events, err := d.store.ListNotificationOutbox(ctx, now, now.Add(-d.lease), 100)
	if err != nil {
		d.logger.WarnContext(ctx, "list notification outbox failed", "error", err)
		return
	}
	for _, event := range events {
		if ctx.Err() != nil {
			return
		}
		claimed, claimErr := d.store.ClaimNotificationOutbox(ctx, event.OrganizationID, event.ID, now)
		if claimErr != nil {
			d.logger.WarnContext(ctx, "claim notification outbox failed", "outbox_id", event.ID, "error", claimErr)
			continue
		}
		if !claimed {
			continue
		}
		d.dispatchOne(ctx, event)
	}
}

func (d *NotificationDispatcher) dispatchOne(ctx context.Context, event pickup.NotificationOutbox) {
	notification, err := d.store.FindNotification(ctx, event.OrganizationID, event.NotificationID)
	if err != nil {
		d.completeFailure(ctx, event, err)
		return
	}

	sent, attempted, deliveryErr := d.deliver(ctx, notification)
	if deliveryErr == nil {
		now := time.Now().UTC()
		_ = d.store.SetNotificationStatus(ctx, event.OrganizationID, pickup.SetNotificationStatusParams{ID: notification.ID, Status: "sent", DeliveryAttempts: event.Attempts + 1, LastAttemptAt: &now, DeliveryError: "", NextRetryAt: nil, SentAt: &now})
		if err := d.store.CompleteNotificationOutbox(ctx, event.OrganizationID, event.ID, "processed", &now, ""); err != nil {
			d.logger.WarnContext(ctx, "complete notification outbox failed", "outbox_id", event.ID, "error", err)
		}
		d.logger.DebugContext(ctx, "notification delivery processed", "notification_id", notification.ID, "sent", sent, "attempted", attempted)
		return
	}

	if event.Attempts+1 >= d.maxAttempts {
		d.completeFailure(ctx, event, deliveryErr)
		return
	}
	next := time.Now().UTC().Add(retryBackoff(event.Attempts))
	_ = d.store.SetNotificationStatus(ctx, event.OrganizationID, pickup.SetNotificationStatusParams{ID: notification.ID, Status: "pending", DeliveryAttempts: event.Attempts + 1, LastAttemptAt: ptrTime(time.Now().UTC()), DeliveryError: truncateTemplateValue(deliveryErr.Error(), 500), NextRetryAt: &next})
	if err := d.store.CompleteNotificationOutbox(ctx, event.OrganizationID, event.ID, "pending", &next, truncateTemplateValue(deliveryErr.Error(), 500)); err != nil {
		d.logger.WarnContext(ctx, "reschedule notification outbox failed", "outbox_id", event.ID, "error", err)
	}
}

func (d *NotificationDispatcher) completeFailure(ctx context.Context, event pickup.NotificationOutbox, deliveryErr error) {
	message := truncateTemplateValue(deliveryErr.Error(), 500)
	now := time.Now().UTC()
	_ = d.store.SetNotificationStatus(ctx, event.OrganizationID, pickup.SetNotificationStatusParams{ID: event.NotificationID, Status: "failed", DeliveryAttempts: event.Attempts + 1, LastAttemptAt: &now, DeliveryError: message, NextRetryAt: nil})
	if err := d.store.CompleteNotificationOutbox(ctx, event.OrganizationID, event.ID, "failed", &now, message); err != nil {
		d.logger.WarnContext(ctx, "mark notification outbox failed", "outbox_id", event.ID, "error", err)
	}
	d.logger.WarnContext(ctx, "wechat notification delivery failed", "notification_id", event.NotificationID, "attempts", event.Attempts+1, "error", deliveryErr)
}

func (d *NotificationDispatcher) deliver(ctx context.Context, notification pickup.Notification) (sent, attempted int, firstErr error) {
	templateKind := notificationTemplateKind(notification.Kind)
	templateID := d.cfg.TemplateForKind(templateKind)
	if templateID == "" {
		return 0, 0, fmt.Errorf("wechat subscribe template is not configured for %s", templateKind)
	}
	accounts, err := d.parents.ListAccountsForStudent(ctx, notification.OrganizationID, notification.StudentID)
	if err != nil {
		return 0, 0, err
	}
	if len(accounts) == 0 {
		return 0, 0, nil
	}
	for _, account := range accounts {
		logItem, logErr := d.store.CreateNotificationDeliveryLog(ctx, notification.OrganizationID, pickup.CreateDeliveryLogParams{NotificationID: notification.ID, ParentAccountID: account.ID, MessageKind: templateKind, TemplateID: templateID})
		if logErr != nil {
			if firstErr == nil {
				firstErr = logErr
			}
			continue
		}
		if logItem.Status == "sent" || logItem.Status == "skipped" {
			continue
		}
		if strings.TrimSpace(account.OpenID) == "" {
			_ = d.store.SetNotificationDeliveryLogStatus(ctx, notification.OrganizationID, pickup.SetDeliveryLogStatusParams{ID: logItem.ID, Status: "skipped", Attempts: logItem.Attempts, DeliveryError: "家长未绑定微信"})
			continue
		}
		subscriptions, subscriptionErr := d.parents.ListMessageSubscriptions(ctx, notification.OrganizationID, account.ID)
		if subscriptionErr != nil {
			if firstErr == nil {
				firstErr = subscriptionErr
			}
			continue
		}
		authorized := false
		for _, subscription := range subscriptions {
			if subscription.Kind == templateKind && subscription.Status == "accept" {
				authorized = true
				break
			}
		}
		if !authorized {
			_ = d.store.SetNotificationDeliveryLogStatus(ctx, notification.OrganizationID, pickup.SetDeliveryLogStatusParams{ID: logItem.ID, Status: "skipped", Attempts: logItem.Attempts, DeliveryError: "家长未授权此类订阅消息"})
			continue
		}

		attempted++
		attemptAt := time.Now().UTC()
		attempts := logItem.Attempts + 1
		_ = d.store.SetNotificationDeliveryLogStatus(ctx, notification.OrganizationID, pickup.SetDeliveryLogStatusParams{ID: logItem.ID, Status: "pending", Attempts: attempts, LastAttemptAt: &attemptAt, DeliveryError: ""})
		sendCtx, cancel := context.WithTimeout(ctx, notificationDeliveryTimeout)
		sendErr := d.sender.SendSubscribeMessage(sendCtx, wechat.SubscribeMessageParams{ToUser: account.OpenID, TemplateID: templateID, Page: d.cfg.SubscribePage, Data: d.cfg.TemplateDataForKind(templateKind, truncateTemplateValue(notification.Title, 20), truncateTemplateValue(notification.Content, 20), notification.CreatedAt)})
		cancel()
		if sendErr != nil {
			if firstErr == nil {
				firstErr = sendErr
			}
			_ = d.store.SetNotificationDeliveryLogStatus(ctx, notification.OrganizationID, pickup.SetDeliveryLogStatusParams{ID: logItem.ID, Status: "failed", Attempts: attempts, LastAttemptAt: &attemptAt, DeliveryError: truncateTemplateValue(sendErr.Error(), 500), NextRetryAt: nil})
			continue
		}
		sent++
		sentAt := time.Now().UTC()
		_ = d.store.SetNotificationDeliveryLogStatus(ctx, notification.OrganizationID, pickup.SetDeliveryLogStatusParams{ID: logItem.ID, Status: "sent", Attempts: attempts, LastAttemptAt: &attemptAt, SentAt: &sentAt, DeliveryError: "", NextRetryAt: nil})
	}
	return sent, attempted, firstErr
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	return time.Duration(attempt*attempt) * 5 * time.Second
}

func ptrTime(value time.Time) *time.Time { return &value }

func notificationTemplateKind(kind string) string {
	switch kind {
	case "pickup_status", "pickup_plan_confirmed", "pickup_change_review":
		return "pickup"
	case "meal_updated", "meal_diet_note_review":
		return "meal"
	case "homework_published", "homework_review":
		return "homework"
	case "leave_review":
		return "leave"
	case "daily_summary_published":
		return "summary"
	default:
		return kind
	}
}

func truncateTemplateValue(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || len([]rune(value)) <= maxRunes {
		return value
	}
	runes := []rune(value)
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}
