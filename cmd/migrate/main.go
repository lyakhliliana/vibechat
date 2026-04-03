package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	appconfig "vibechat/internal/config"
	"vibechat/internal/infrastructure/storage"
	"vibechat/utils/logger"
)

func main() {
	cfgPath := flag.String("config", "configs/migrate.yaml", "path to migrate config file")
	flag.Parse()

	cfg, err := appconfig.LoadMigrate(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.Logger)

	var dsn, migrationsPath string

	switch cfg.Storage.Type {
	case storage.TypePostgres:
		p := cfg.Storage.Postgres
		if p == nil {
			fmt.Fprintln(os.Stderr, "postgres config is required for type postgres")
			os.Exit(1)
		}
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			p.User, p.Password, p.Host, p.Port, p.DBName, p.SSLMode,
		)
		migrationsPath = p.MigrationsPath

	case storage.TypeMySQL:
		m := cfg.Storage.MySQL
		if m == nil {
			fmt.Fprintln(os.Stderr, "mysql config is required for type mysql")
			os.Exit(1)
		}
		dsn = m.MigrateDSN()
		migrationsPath = m.MigrationsPath

	default:
		fmt.Fprintf(os.Stderr, "unsupported storage type %q\n", cfg.Storage.Type)
		os.Exit(1)
	}

	mig, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
	defer mig.Close()

	if err = mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("migrations applied")
}
