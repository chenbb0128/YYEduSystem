package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	assignmentmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment/mysqlrepo"
	auditmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/audit"
	auditmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/audit/mysqlrepo"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/homework"
	homeworkmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/homework/mysqlrepo"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	identitymysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/identity/mysqlrepo"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	masterdatamysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata/mysqlrepo"
	mealmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/meal"
	mealmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/meal/mysqlrepo"
	mediamodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/media"
	mediamysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/media/mysqlrepo"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	parentmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/parent/mysqlrepo"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	pickupmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup/mysqlrepo"
	platformadmin "github.com/chenbb0128/tuoguan-system-server/internal/modules/platformadmin"
	platformadminmysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/platformadmin/mysqlrepo"
	reportmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/report"
	schedulemodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/schedule"
	schedulemysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/schedule/mysqlrepo"
	summarymodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/summary"
	summarymysqlrepo "github.com/chenbb0128/tuoguan-system-server/internal/modules/summary/mysqlrepo"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	platformmetrics "github.com/chenbb0128/tuoguan-system-server/internal/platform/metrics"
	redisclient "github.com/chenbb0128/tuoguan-system-server/internal/platform/redis"
	smsplatform "github.com/chenbb0128/tuoguan-system-server/internal/platform/sms"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/verification"
	wechatclient "github.com/chenbb0128/tuoguan-system-server/internal/platform/wechat"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/middleware"
)

type API struct {
	server          *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
	database        *database.DB
	redis           *redisclient.Client
}

func NewAPI(cfg config.Config, logger *slog.Logger) (*API, error) {
	var db *database.DB
	var redis *redisclient.Client
	var metrics *platformmetrics.Metrics
	var checks []httpapi.ReadyCheck
	var masterDataStore masterdata.Store
	var pickupStore pickup.Store
	var parentStore parent.Store
	var assignmentStore assignment.Store
	var homeworkStore homework.Store
	var mealStore mealmodule.Store
	var summaryStore summarymodule.Store
	var scheduleStore schedulemodule.Store
	var userStore identity.UserStore
	var auditStore auditmodule.Store
	var mediaStore mediamodule.Store
	var platformStore platformadmin.Store

	if cfg.Database.Enabled {
		openCtx, cancel := context.WithTimeout(context.Background(), cfg.Database.PingTimeout)
		defer cancel()

		opened, err := database.Open(openCtx, cfg.Database)
		if err != nil {
			return nil, err
		}
		db = opened
		masterDataStore = masterdatamysqlrepo.New(db.SQL)
		pickupStore = pickupmysqlrepo.New(db.SQL)
		scheduleStore = schedulemysqlrepo.New(db.SQL)
		parentStore = parentmysqlrepo.New(db.SQL)
		assignmentStore = assignmentmysqlrepo.New(db.SQL)
		homeworkStore = homeworkmysqlrepo.New(db.SQL)
		mealStore = mealmysqlrepo.New(db.SQL)
		summaryStore = summarymysqlrepo.New(db.SQL)
		userStore = identitymysqlrepo.New(db.SQL)
		auditStore = auditmysqlrepo.New(db.SQL)
		mediaStore = mediamysqlrepo.New(db.SQL)
		platformStore = platformadminmysqlrepo.New(db.SQL)
		checks = append(checks, httpapi.ReadyCheck{
			Name: "mysql",
			Check: func(ctx context.Context) error {
				pingCtx, cancel := context.WithTimeout(ctx, cfg.Database.PingTimeout)
				defer cancel()
				return db.Ping(pingCtx)
			},
		})
	}
	if masterDataStore == nil {
		masterDataStore = masterdata.NewMemoryStore()
	}
	if pickupStore == nil {
		pickupStore = pickup.NewMemoryStore()
	}
	if scheduleStore == nil {
		scheduleStore = schedulemodule.NewMemoryStore()
	}
	if parentStore == nil {
		parentStore = parent.NewMemoryStore()
	}
	if assignmentStore == nil {
		assignmentStore = assignment.NewMemoryStore()
	}
	if homeworkStore == nil {
		homeworkStore = homework.NewMemoryStore()
	}
	if mealStore == nil {
		mealStore = mealmodule.NewMemoryStore()
	}
	if summaryStore == nil {
		summaryStore = summarymodule.NewMemoryStore()
	}
	if userStore == nil {
		userStore = identity.NewMemoryStore()
	}
	if auditStore == nil {
		auditStore = auditmodule.NewMemoryStore()
	}
	if mediaStore == nil {
		mediaStore = mediamodule.NewMemoryStore()
	}
	if platformStore == nil {
		platformStore = platformadmin.NewMemoryStore()
	}
	if cfg.Auth.BootstrapAdminEnabled {
		if err := identity.EnsureConfiguredAdmin(context.Background(), userStore, cfg.Auth.BootstrapAdminUsername, cfg.Auth.BootstrapAdminPassword); err != nil {
			if db != nil {
				_ = db.Close()
			}
			return nil, fmt.Errorf("bootstrap admin user: %w", err)
		}
	}
	if cfg.Auth.BootstrapPlatformAdminEnabled {
		if err := identity.EnsureConfiguredPlatformAdmin(context.Background(), userStore, cfg.Auth.BootstrapPlatformAdminUsername, cfg.Auth.BootstrapPlatformAdminPassword); err != nil {
			if db != nil {
				_ = db.Close()
			}
			return nil, fmt.Errorf("bootstrap platform admin user: %w", err)
		}
	}
	secret := cfg.Auth.Secret
	if secret == "" {
		secret = config.DefaultAuthSecret
	}
	accessTTL := cfg.Auth.AccessTTL
	if accessTTL <= 0 {
		accessTTL = 2 * time.Hour
	}
	refreshTTL := cfg.Auth.RefreshTTL
	if refreshTTL <= accessTTL {
		refreshTTL = 30 * 24 * time.Hour
	}
	tokens, err := identity.NewTokenManager(secret, accessTTL, refreshTTL)
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, err
	}
	identityHandler := identity.NewHandler(userStore, tokens)
	platformHandler := platformadmin.NewHandler(platformStore, userStore)
	masterDataHandler := masterdata.NewHandler(masterDataStore)
	var photoStore storage.Store
	var uploadReader storage.FileReader
	maxFileBytes := cfg.Storage.MaxFileBytes
	if maxFileBytes <= 0 {
		maxFileBytes = storage.DefaultMaxPhotoBytes
	}
	switch provider := strings.ToLower(strings.TrimSpace(cfg.Storage.Provider)); provider {
	case "", "local":
		uploadDir := cfg.Storage.UploadDir
		if strings.TrimSpace(uploadDir) == "" {
			uploadDir = "data/uploads"
		}
		localStore, storageErr := storage.NewLocalFileStore(uploadDir, "/uploads", maxFileBytes)
		if storageErr != nil {
			if db != nil {
				_ = db.Close()
			}
			return nil, storageErr
		}
		photoStore, uploadReader = localStore, localStore
	case "s3":
		s3Store, storageErr := storage.NewS3Store(storage.S3Config{
			Endpoint: cfg.Storage.Endpoint, Bucket: cfg.Storage.Bucket, Region: cfg.Storage.Region,
			AccessKey: cfg.Storage.AccessKey, SecretKey: cfg.Storage.SecretKey,
			PathStyle: cfg.Storage.PathStyle, PublicPath: "/uploads", MaxBytes: maxFileBytes,
		})
		if storageErr != nil {
			if db != nil {
				_ = db.Close()
			}
			return nil, storageErr
		}
		photoStore, uploadReader = s3Store, s3Store
	default:
		if db != nil {
			_ = db.Close()
		}
		return nil, fmt.Errorf("unsupported storage provider %q", provider)
	}
	pickupHandler := pickup.NewHandler(pickupStore, masterDataStore, photoStore, parentStore)
	pickupHandler.SetStaffScope(assignmentStore, userStore)
	pickupHandler.SetAuditWriter(auditStore)
	assignmentHandler := assignment.NewHandler(assignmentStore, userStore, masterDataStore)
	homeworkHandler := homework.NewHandler(homeworkStore, masterDataStore)
	homeworkHandler.SetStaffScope(assignmentStore, userStore)
	homeworkHandler.SetPhotoStore(photoStore)
	homeworkHandler.SetParentStore(parentStore)
	homeworkHandler.SetNotificationWriter(pickupStore)
	homeworkHandler.SetAuditWriter(auditStore)
	mealHandler := mealmodule.NewHandler(mealStore, masterDataStore)
	mealHandler.SetParentStore(parentStore)
	mealHandler.SetNotificationWriter(pickupStore)
	mealHandler.SetStaffScope(assignmentStore)
	mealHandler.SetPhotoStore(photoStore)
	mealHandler.SetAuditWriter(auditStore)
	summaryHandler := summarymodule.NewHandler(summaryStore, pickupStore, homeworkStore, mealStore, masterDataStore)
	summaryHandler.SetParentStore(parentStore)
	summaryHandler.SetNotificationWriter(pickupStore)
	summaryHandler.SetStaffScope(assignmentStore)
	summaryHandler.SetAuditWriter(auditStore)
	scheduleHandler := schedulemodule.NewHandler(scheduleStore, masterDataStore, assignmentStore, userStore, pickupStore)
	parentHandler := parent.NewHandler(parentStore, masterDataStore, pickupStore, tokens)
	parentHandler.SetStaffScope(assignmentStore)
	parentHandler.SetUserStore(userStore)
	parentHandler.SetAuditWriter(auditStore)
	auditHandler := auditmodule.NewHandler(auditStore, masterdata.DefaultOrganizationID)
	reportHandler := reportmodule.NewHandler(pickupStore, homeworkStore, mealStore, parentStore, masterDataStore, summaryStore, assignmentStore)
	photoSigningSecret := strings.TrimSpace(cfg.Storage.URLSigningSecret)
	if photoSigningSecret == "" {
		photoSigningSecret = secret
	}
	photoSigner, err := storage.NewURLSigner(photoSigningSecret)
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, err
	}
	pickupHandler.SetPhotoSigner(photoSigner)
	pickupHandler.SetPhotoURLTTL(cfg.Storage.SignedURLTTL)
	pickupHandler.SetAssetStore(mediaStore)
	pickupHandler.SetAssetRetentionDays(cfg.Storage.RetentionDays)
	homeworkHandler.SetPhotoSigner(photoSigner)
	homeworkHandler.SetPhotoURLTTL(cfg.Storage.SignedURLTTL)
	homeworkHandler.SetAssetStore(mediaStore)
	homeworkHandler.SetAssetRetentionDays(cfg.Storage.RetentionDays)
	parentHandler.SetPhotoSigner(photoSigner)
	parentHandler.SetPhotoURLTTL(cfg.Storage.SignedURLTTL)
	mealHandler.SetPhotoSigner(photoSigner)
	mealHandler.SetPhotoURLTTL(cfg.Storage.SignedURLTTL)
	mealHandler.SetAssetStore(mediaStore)
	mealHandler.SetAssetRetentionDays(cfg.Storage.RetentionDays)
	parentHandler.SetAllowLocalCode(!strings.EqualFold(cfg.App.Env, "prod") && !strings.EqualFold(cfg.App.Env, "production"))
	parentHandler.SetAllowLocalPhoneCode(!strings.EqualFold(cfg.App.Env, "prod") && !strings.EqualFold(cfg.App.Env, "production"))
	if cfg.WeChat.Enabled {
		createdWechat, wechatErr := wechatclient.NewClient(cfg.WeChat.AppID, cfg.WeChat.Secret, cfg.WeChat.Endpoint, cfg.WeChat.Timeout)
		if wechatErr != nil {
			if db != nil {
				_ = db.Close()
			}
			return nil, fmt.Errorf("configure wechat login: %w", wechatErr)
		}
		parentHandler.SetCodeExchanger(createdWechat)
	}
	if cfg.Redis.Enabled {
		openCtx, cancel := context.WithTimeout(context.Background(), cfg.Redis.PingTimeout)
		defer cancel()

		opened, err := redisclient.Open(openCtx, cfg.Redis)
		if err != nil {
			if db != nil {
				_ = db.Close()
			}
			return nil, err
		}
		redis = opened
		checks = append(checks, httpapi.ReadyCheck{
			Name: "redis",
			Check: func(ctx context.Context) error {
				pingCtx, cancel := context.WithTimeout(ctx, cfg.Redis.PingTimeout)
				defer cancel()
				return redis.Ping(pingCtx)
			},
		})
	}
	verificationStore := verification.Store(verification.NewMemoryStore())
	if redis != nil {
		createdVerificationStore, storeErr := verification.NewRedisStore(redis)
		if storeErr != nil {
			if db != nil {
				_ = db.Close()
			}
			_ = redis.Close()
			return nil, storeErr
		}
		verificationStore = createdVerificationStore
	}
	phoneCodeSender, senderErr := smsplatform.NewSender(cfg.SMS)
	if senderErr != nil {
		if db != nil {
			_ = db.Close()
		}
		if redis != nil {
			_ = redis.Close()
		}
		return nil, fmt.Errorf("configure SMS sender: %w", senderErr)
	}
	phoneCodeService, serviceErr := verification.NewService(verificationStore, phoneCodeSender, cfg.SMS, secret)
	if serviceErr != nil {
		if db != nil {
			_ = db.Close()
		}
		if redis != nil {
			_ = redis.Close()
		}
		return nil, fmt.Errorf("configure phone verification: %w", serviceErr)
	}
	parentHandler.SetPhoneCodeService(phoneCodeService)

	if cfg.Observability.Metrics.Enabled {
		created, err := platformmetrics.New(cfg.Observability.Metrics)
		if err != nil {
			if db != nil {
				_ = db.Close()
			}
			if redis != nil {
				_ = redis.Close()
			}
			return nil, err
		}
		metrics = created
	}

	router, err := httpapi.NewRouter(httpapi.RouterOptions{
		App:             cfg.App,
		HTTP:            cfg.HTTP,
		Logger:          logger,
		ReadyTimeout:    readinessTimeout(cfg),
		ReadinessChecks: checks,
		Metrics:         metrics,
		RegisterAPIRoutes: func(apiGroup *gin.RouterGroup) {
			identityHandler.RegisterPublicRoutes(apiGroup)
			parentHandler.RegisterAuthRoutes(apiGroup)
			platformHandler.RegisterPublicRoutes(apiGroup)

			authenticated := apiGroup.Group("")
			authenticated.Use(middleware.Authenticate(tokens, userStore))
			identityHandler.RegisterAuthenticatedRoutes(authenticated)

			staff := authenticated.Group("")
			staff.Use(middleware.RequireStaff())
			identityHandler.RegisterStaffRoutes(staff)
			auditHandler.RegisterRoutes(staff)
			assignmentHandler.RegisterRoutes(staff)
			scheduleHandler.RegisterRoutes(staff)
			homeworkHandler.RegisterStaffRoutes(staff)
			masterDataRead := staff.Group("")
			masterDataRead.Use(teacherMasterDataScope(assignmentStore))
			masterDataHandler.RegisterReadRoutes(masterDataRead)
			masterDataWrite := staff.Group("")
			masterDataWrite.Use(middleware.RequireManager())
			masterDataHandler.RegisterWriteRoutes(masterDataWrite)
			pickupHandler.RegisterRoutes(staff)
			parentHandler.RegisterStaffRoutes(staff)
			mealHandler.RegisterStaffRoutes(staff)
			summaryHandler.RegisterStaffRoutes(staff)
			reportHandler.RegisterRoutes(staff)

			platform := authenticated.Group("")
			platform.Use(middleware.RequirePlatformAdmin())
			platformHandler.RegisterRoutes(platform)

			parents := authenticated.Group("")
			parents.Use(middleware.RequireParent())
			homeworkHandler.RegisterParentRoutes(parents)
			parentHandler.RegisterParentRoutes(parents)
			mealHandler.RegisterParentRoutes(parents)
			summaryHandler.RegisterParentRoutes(parents)
		},
		UploadsHandler: protectedUploadHandler(photoSigner, uploadReader),
	})
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		if redis != nil {
			_ = redis.Close()
		}
		return nil, err
	}

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}

	return &API{
		server:          server,
		logger:          logger,
		shutdownTimeout: cfg.HTTP.ShutdownTimeout,
		database:        db,
		redis:           redis,
	}, nil
}

func (a *API) Run(ctx context.Context) (err error) {
	defer func() {
		if closeErr := a.close(); closeErr != nil {
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("api listening", "addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	a.logger.Info("api shutting down", "timeout", a.shutdownTimeout.String())
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown api: %w", err)
	}
	return <-errCh
}

func (a *API) close() error {
	var err error
	if a.database != nil {
		if closeErr := a.database.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close mysql: %w", closeErr))
		}
	}
	if a.redis != nil {
		if closeErr := a.redis.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close redis: %w", closeErr))
		}
	}
	return err
}

func readinessTimeout(cfg config.Config) time.Duration {
	timeout := 2 * time.Second
	if cfg.Database.Enabled && cfg.Database.PingTimeout > timeout {
		timeout = cfg.Database.PingTimeout
	}
	if cfg.Redis.Enabled && cfg.Redis.PingTimeout > timeout {
		timeout = cfg.Redis.PingTimeout
	}
	return timeout
}
