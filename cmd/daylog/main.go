package main

import (
	"daylog/internal"
	"flag"
	"github.com/google/uuid"
	"log"
	"os"
	"strings"
)

var (
	defaultPort     = 3000
	defaultDatabase = "daylog.db"
)

func RandomSessionKey() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)[0:32]
}

func MakeConfig() *internal.Config {
	port := flag.Int("port", defaultPort, "HTTP Port for the App")
	dbFileName := flag.String("db", defaultDatabase, "Database file name")

	flag.Parse()

	sessionKey := os.Getenv("SESSION_KEY")

	if sessionKey == "" {
		log.Println("No SESSION_KEY environment variable set; Using random session key")
		sessionKey = RandomSessionKey()
	}

	authSecret := os.Getenv("AUTH_SECRET")

	if authSecret == "" {
		log.Println("No AUTH_SECRET environment variable set")
		return nil
	}

	config := &internal.Config{
		Port:       *port,
		DBFileName: *dbFileName,
		SessionKey: sessionKey,
		AuthSecret: authSecret,
	}

	return config
}

func main() {
	config := MakeConfig()

	if config == nil {
		log.Fatal("No valid config")
	}

	internal.Run(*config)
}
