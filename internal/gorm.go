package internal

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func ModelsToMigrate() []interface{} {
	return []interface{}{
		&Post{},
	}
}

func NewDB(dbPath string) (*gorm.DB, error) {
	dbc, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})

	if err != nil {
		return nil, err
	}

	if err := dbc.AutoMigrate(ModelsToMigrate()...); err != nil {
		return nil, err
	}

	return dbc, nil
}
