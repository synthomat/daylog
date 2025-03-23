package internal

import (
	"embed"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
)

type Server struct {
	Db     *gorm.DB
	Engine *gin.Engine
}

//go:embed all:static
var assetsFS embed.FS

func Run(config Config) {
	db, err := NewDB(config.DBFileName)

	if err != nil {
		slog.Error("Could not open database: %s\n", err.Error())
		return
	}

	dbInst, _ := db.DB()
	defer dbInst.Close()

	r := gin.Default()
	r.HTMLRender = DefaultPongo2(gin.IsDebugging())

	var store = cookie.NewStore([]byte(config.SessionKey))
	store.Options(sessions.Options{
		Path: "/",
	})

	r.Use(sessions.Sessions("dl-sess", store))
	r.Use(AuthMiddleware())

	assetsFS, _ := fs.Sub(assetsFS, "static")
	r.StaticFS("/static", http.FS(assetsFS))
	r.Static("/uploads", "./uploads").Use(AuthMiddleware())

	r.Match([]string{http.MethodGet, http.MethodPost}, "/login", LoginHandler(db, config))
	r.POST("/logout", LogoutHandler(db))

	r.GET("/", IndexHandler(db))

	r.Match([]string{http.MethodGet, http.MethodPost}, "/new", NewPostHandler(db))
	pg := r.Group("/posts/:pid").Use(InjectPostMiddleware(db))
	{
		pg.DELETE("/", DeletePostHandler(db))
		pg.Match([]string{http.MethodGet, http.MethodPost}, "/edit", EditPostHandler(db))
	}

	r.POST("/upload", UploadFileHandler(db))

	err = r.Run(fmt.Sprintf("0.0.0.0:%d", config.Port))

	if err != nil {
		log.Fatalf("Could not start application: %s\n", err.Error())
		return
	}
}
