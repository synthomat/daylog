package internal

import (
	"embed"
	"errors"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"log/slog"
)

//go:embed all:migrations/*.sql
var mfs embed.FS

func RunMigrations(dbc *gorm.DB) {
	slog.Debug("Loading migrations...")
	source, err := iofs.New(mfs, "migrations")

	if err != nil {
		log.Fatal(err)
	}

	db, _ := dbc.DB()
	dbDriver, _ := sqlite3.WithInstance(db, &sqlite3.Config{})

	m, err := migrate.NewWithInstance("iofs", source, "daylog", dbDriver)
	//defer m.Close()

	slog.Debug("Running migrations...")
	err = m.Up()

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic(err)
	}
}

func NewDB(dbPath string) (*gorm.DB, error) {
	dbc, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})

	if err != nil {
		return nil, err
	}

	RunMigrations(dbc)

	return dbc, nil
}
