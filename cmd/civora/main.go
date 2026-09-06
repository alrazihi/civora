package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	auditapi "github.com/alrazihi/civora/internal/audit/api"
	auditapp "github.com/alrazihi/civora/internal/audit/application"
	auditpostgres "github.com/alrazihi/civora/internal/audit/infrastructure/postgres"
	caseapi "github.com/alrazihi/civora/internal/cases/api"
	caseapp "github.com/alrazihi/civora/internal/cases/application"
	casepostgres "github.com/alrazihi/civora/internal/cases/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/config"
	"github.com/alrazihi/civora/internal/database"
	identityapi "github.com/alrazihi/civora/internal/identity/api"
	identityapp "github.com/alrazihi/civora/internal/identity/application"
	"github.com/alrazihi/civora/internal/identity/domain"
	identityauth "github.com/alrazihi/civora/internal/identity/infrastructure/auth"
	identitypostgres "github.com/alrazihi/civora/internal/identity/infrastructure/postgres"
	intmid "github.com/alrazihi/civora/internal/middleware"
	orgapi "github.com/alrazihi/civora/internal/organizations/api"
	orgapp "github.com/alrazihi/civora/internal/organizations/application"
	orgpostgres "github.com/alrazihi/civora/internal/organizations/infrastructure/postgres"
	"github.com/alrazihi/civora/internal/server"
	"github.com/alrazihi/civora/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	dsn := database.BuildDSN(cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode)
	db, err := database.NewDatabase(dsn, cfg.Database.Driver)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	migrator := database.NewMigrator(db.DB, migrations.FS)
	if err := migrator.LoadMigrations(); err != nil {
		log.Fatalf("failed to load migrations: %v", err)
	}
	ctx := context.Background()
	if err := migrator.Migrate(ctx); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	userRepo := identitypostgres.NewPostgresUserRepository(db.DB)
	roleRepo := identitypostgres.NewPostgresRoleRepository(db.DB)
	tokenSvc := identityauth.NewJWTTokenService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry, "civora")
	hasher := domain.NewBCryptHasher(cfg.Auth.BCryptCost)

	orgRepo := orgpostgres.NewPostgresOrganizationRepository(db.DB)
	caseRepo := casepostgres.NewPostgresCaseRepository(db.DB)
	auditRepo := auditpostgres.NewPostgresAuditRepository(db.DB)

	auditService := auditapp.NewAuditService(auditRepo)

	jwtSvc := intmid.NewJWTService(cfg.Auth.JWTSecret)
	authMiddleware := intmid.AuthRequired(jwtSvc)

	identityService := identityapp.NewIdentityService(userRepo, roleRepo, hasher, tokenSvc, auditService)
	identityHandler := identityapi.NewHandler(identityService)

	orgService := orgapp.NewOrganizationService(orgRepo, auditService)
	orgHandler := orgapi.NewHandler(orgService)

	caseService := caseapp.NewCaseService(caseRepo, auditService)
	caseHandler := caseapi.NewHandler(caseService)

	auditHandler := auditapi.NewHandler(auditService)

	srv := server.New(cfg)
	identityHandler.RegisterRoutes(srv.Router(), authMiddleware)
	orgHandler.RegisterRoutes(srv.Router(), authMiddleware)
	caseHandler.RegisterRoutes(srv.Router(), authMiddleware)
	auditHandler.RegisterRoutes(srv.Router(), authMiddleware)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("CIVORA starting on port %s", cfg.Server.Port)
	if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
