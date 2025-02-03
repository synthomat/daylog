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
)

//go:embed all:migrations/*.sql
var mfs embed.FS

func ModelsToMigrate() []interface{} {
	return []interface{}{
		&Post{},
	}
}

func RunMigrations(dbc *gorm.DB) {
	db, _ := dbc.DB()

	d, err := iofs.New(mfs, "migrations")
	if err != nil {
		log.Fatal(err)
	}

	dbDriver, _ := sqlite3.WithInstance(db, &sqlite3.Config{})

	m, err := migrate.NewWithInstance("iofs", d, "daylog.db", dbDriver)

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

	/*
		if err := dbc.AutoMigrate(ModelsToMigrate()...); err != nil {
			return nil, err
		}

	*/

	RunMigrations(dbc)

	return dbc, nil
}
