package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	masterdatamysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata/mysqlrepo"
	parentmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/parent/mysqlrepo"
	pickupmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup/mysqlrepo"
	schedulemodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/schedule"
	schedulemysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/schedule/mysqlrepo"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/queue"
	redisclient "github.com/chenbb0128/tuoguan-system-server/internal/platform/redis"
	wechatclient "github.com/chenbb0128/tuoguan-system-server/internal/platform/wechat"
	"github.com/chenbb0128/tuoguan-system-server/internal/workers"
)

type Worker struct {
	cfg    config.Config
	logger *slog.Logger
}

func NewWorker(cfg config.Config, logger *slog.Logger) *Worker {
	return &Worker{cfg: cfg, logger: logger}
}

func (w *Worker) Run(ctx context.Context) (err error) {
	if !w.cfg.Worker.Enabled {
		w.logger.Info("worker disabled", "app", w.cfg.App.Name)
		<-ctx.Done()
		w.logger.Info("worker stopped")
		return nil
	}

	openCtx, cancel := context.WithTimeout(context.Background(), w.cfg.Redis.PingTimeout)
	defer cancel()

	redis, err := redisclient.Open(openCtx, w.cfg.Redis)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := redis.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close redis: %w", closeErr))
		}
	}()

	var databaseConn *database.DB
	if w.cfg.Database.Enabled {
		openDatabaseCtx, cancelDatabase := context.WithTimeout(context.Background(), w.cfg.Database.PingTimeout)
		databaseConn, err = database.Open(openDatabaseCtx, w.cfg.Database)
		cancelDatabase()
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := databaseConn.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close mysql: %w", closeErr))
			}
		}()
	}

	server := queue.NewServer(w.cfg.Redis, w.cfg.Worker, w.logger)
	mux := workers.NewMux()

	w.logger.Info(
		"worker starting",
		"concurrency", w.cfg.Worker.Concurrency,
		"queues", w.cfg.Worker.Queues,
		"shutdown_timeout", w.cfg.Worker.ShutdownTimeout.String(),
	)
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}

	var dispatcherDone chan struct{}
	var scheduleDone chan struct{}
	if databaseConn != nil && w.cfg.WeChat.Enabled && w.cfg.WeChat.HasSubscribeTemplates() {
		sender, senderErr := wechatclient.NewClient(w.cfg.WeChat.AppID, w.cfg.WeChat.Secret, w.cfg.WeChat.Endpoint, w.cfg.WeChat.Timeout)
		if senderErr != nil {
			return fmt.Errorf("configure wechat delivery: %w", senderErr)
		}
		pickupStore := pickupmysqlrepo.New(databaseConn.SQL)
		parentStore := parentmysqlrepo.New(databaseConn.SQL)
		dispatcher := NewNotificationDispatcher(pickupStore, parentStore, sender, w.cfg.WeChat, w.logger, w.cfg.Worker.NotificationPollInterval, w.cfg.Worker.NotificationLease, w.cfg.Worker.NotificationMaxAttempts)
		dispatcherDone = make(chan struct{})
		go func() {
			defer close(dispatcherDone)
			dispatcher.Run(ctx)
		}()
		w.logger.Info("notification outbox dispatcher started", "poll_interval", w.cfg.Worker.NotificationPollInterval.String(), "lease", w.cfg.Worker.NotificationLease.String(), "max_attempts", w.cfg.Worker.NotificationMaxAttempts)
	} else if w.cfg.Database.Enabled {
		w.logger.Info("notification outbox dispatcher not started", "wechat_enabled", w.cfg.WeChat.Enabled, "templates_configured", w.cfg.WeChat.HasSubscribeTemplates())
	}
	if databaseConn != nil {
		masterDataStore := masterdatamysqlrepo.New(databaseConn.SQL)
		pickupStore := pickupmysqlrepo.New(databaseConn.SQL)
		scheduleStore := schedulemysqlrepo.New(databaseConn.SQL)
		generator := schedulemodule.NewGenerator(scheduleStore, masterDataStore, pickupStore)
		scheduleDone = make(chan struct{})
		go func() {
			defer close(scheduleDone)
			runScheduleGeneration(ctx, generator, w.cfg.Worker.ScheduleGenerationInterval, w.logger)
		}()
		w.logger.Info("pickup schedule generator started", "interval", w.cfg.Worker.ScheduleGenerationInterval.String())
	}

	<-ctx.Done()
	w.logger.Info("worker shutting down")
	server.Shutdown()
	if dispatcherDone != nil {
		<-dispatcherDone
	}
	if scheduleDone != nil {
		<-scheduleDone
	}
	w.logger.Info("worker stopped")
	return nil
}

func runScheduleGeneration(ctx context.Context, generator *schedulemodule.Generator, interval time.Duration, logger *slog.Logger) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	generate := func() {
		date, err := localDate()
		if err != nil {
			logger.Warn("resolve local date for pickup schedules failed", "error", err)
			return
		}
		result, err := generator.GenerateForDate(ctx, masterdata.DefaultOrganizationID, date)
		if err != nil {
			logger.Warn("generate pickup schedules failed", "date", date.Format("2006-01-02"), "error", err)
			return
		}
		if len(result.CreatedOperationIDs) > 0 {
			logger.Info("generated pickup operations from schedules", "date", date.Format("2006-01-02"), "count", len(result.CreatedOperationIDs))
		}
	}
	generate()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			generate()
		}
	}
}

func localDate() (time.Time, error) {
	return time.ParseInLocation("2006-01-02", time.Now().Format("2006-01-02"), time.UTC)
}
