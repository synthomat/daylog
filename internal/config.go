package internal

type Config struct {
	// Web Port
	Port int

	// Database file name
	DBFileName string

	SessionKey string

	AuthSecret string
}
