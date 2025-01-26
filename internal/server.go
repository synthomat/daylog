package internal

import (
	"embed"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var sessionName = "dsession"

type Server struct {
	Db     *gorm.DB
	Engine *gin.Engine
}

//go:embed all:static
var assetsFS embed.FS

type PaginationLink struct {
	Ord    int64
	Params string
}

func Run(config Config) {
	db, err := NewDB(config.DBFileName)

	if err != nil {
		log.Fatalf("Could not open database: %s\n", err.Error())
	}

	r := gin.Default()
	r.HTMLRender = DefaultPongo2(gin.IsDebugging())

	var store = cookie.NewStore([]byte(config.SessionKey))
	r.Use(sessions.Sessions("dlsess", store))

	r.Use(AuthMiddleware())

	assetsFS, _ := fs.Sub(assetsFS, "static")
	r.StaticFS("/static", http.FS(assetsFS))

	authController := AuthController{
		db:     db,
		config: &config,
	}

	r.Any("/login", authController.LoginHandler)
	r.POST("/logout", authController.LogoutHandler)

	postController := PostController{
		db: db,
	}

	r.GET("/", func(c *gin.Context) {
		type YearEntries struct {
			Year     int
			Count    int
			IsActive bool
		}

		yearQuery := c.Query("year")
		searchQuery := strings.TrimSpace(c.Query("q"))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))

		if page < 1 {
			page = 1
		}

		if yearQuery == "" {
			year, _, _ := time.Now().Date()
			yearQuery = strconv.Itoa(year)
		}

		yearInt, _ := strconv.Atoi(yearQuery)

		var posts []Post

		query := db.Order("event_time desc")

		if searchQuery != "" {
			query.Where("lower(body) like ?", "%"+strings.ToLower(searchQuery)+"%")
		}

		if searchQuery == "" && yearQuery != "" {
			query.Where("strftime('%Y', event_time) = ?", yearQuery)
		}

		var totalCount int64
		query.Find(&posts).Count(&totalCount)
		query.Offset((page - 1) * 10).Limit(10).Find(&posts)

		var yearEntries []YearEntries
		db.Raw("SELECT DISTINCT strftime('%Y', event_time) as year, count(*) as count\n" +
			"FROM posts\n" +
			"GROUP BY year\n" +
			"ORDER BY year DESC").Scan(&yearEntries)

		var paginationLinks []PaginationLink

		var pages []int64

		q := c.Request.URL.Query()

		for p := int64(1); p <= totalCount/10; p++ {
			q.Set("page", strconv.FormatInt(p, 10))

			pl := PaginationLink{
				Ord:    p,
				Params: q.Encode(),
			}

			paginationLinks = append(paginationLinks, pl)
		}

		c.HTML(http.StatusOK, "index.html", Rcx(c, Cx{
			"posts":       posts,
			"searchQuery": searchQuery,
			"years":       yearEntries,
			"yearFilter":  yearInt,
			"totalCount":  totalCount,
			"perPage":     10,
			"page":        page,
			"pages":       pages,
			"pagination":  paginationLinks,
		}))
	})

	r.Any("/new", postController.NewPostHandler)
	pg := r.Group("/posts/:pid").Use(InjectPost(db))
	{
		pg.Any("/edit", postController.EditPostHandler)
		pg.DELETE("/", postController.DeletePostHandler)
	}

	err = r.Run("0.0.0.0:" + strconv.Itoa(config.Port))

	if err != nil {
		log.Fatalf("Could not start application: %s\n", err.Error())
		return
	}
}
