package internal

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

func ModelsToMigrate() []interface{} {
	return []interface{}{
		&Post{},
	}
}

func NewDB(dbPath string) (*gorm.DB, error) {
	dbc, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic("failed to open database: " + err.Error())
	}

	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	if err := dbc.AutoMigrate(ModelsToMigrate()...); err != nil {
		log.Fatalln(err)
		return nil, err
	}

	return dbc, nil
}
