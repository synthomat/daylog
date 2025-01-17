package internal

import (
	"embed"
	"errors"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io/fs"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var sessionName = "dsession"

// Parses the request and returns a Post struct
func postFromRequest(r *http.Request) (*Post, error) {
	eventTime, _ := time.Parse("2006-01-02T15:04", r.FormValue("event_time"))

	title := r.FormValue("title")

	post := Post{
		EventTime: eventTime,
		Title:     &title,
		Body:      r.FormValue("body"),
	}

	return &post, nil
}

type Server struct {
	Db     *gorm.DB
	Engine *gin.Engine
}

func InjectPost(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		postId := c.Param("pid")
		postUuid := uuid.MustParse(postId)

		post := Post{
			BaseModel: BaseModel{Id: postUuid},
		}

		err := db.First(&post).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, "Post not found")
			return
		}

		c.Set("post", post)
		c.Next()
	}
}

func postFromCtx(c *gin.Context) Post {
	return c.MustGet("post").(Post)
}

func AuthMiddleware() gin.HandlerFunc {
	rex, err := regexp.Compile("^/(static|login|logout)")

	if err != nil {
		panic(err)
	}

	return func(c *gin.Context) {
		session := sessions.Default(c)
		/*
			if err != nil {
				fmt.Print(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		*/

		if rex.Match([]byte(c.Request.URL.Path)) {
			c.Next()
			return
		}

		if session.Get("authenticated") == nil {
			c.Header("HX-Redirect", "/login")
			c.Redirect(http.StatusFound, "/login")
			return
		}

		c.Set("authenticated", true)
		c.Next()
	}
}

type AuthController struct {
	db     *gorm.DB
	config *Config
}

func (ac *AuthController) LoginHandler(c *gin.Context) {
	session := sessions.Default(c)

	if c.Request.Method != http.MethodPost {
		c.HTML(http.StatusOK, "login.html", nil)
		return
	}

	password := c.PostForm("password")

	if password == ac.config.AuthSecret {
		session.Set("authenticated", true)

		err := session.Save()

		if err != nil {
			fmt.Println(err.Error())
			c.AbortWithError(http.StatusInternalServerError, err)
		}

		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "login.html", nil)
}

func (ac *AuthController) LogoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.Header("HX-Redirect", "/")
	c.Header("Location", "/")
	c.String(http.StatusOK, "")
}

type PostController struct {
	db *gorm.DB
}

func (pc *PostController) EditPostHandler(c *gin.Context) {
	post := postFromCtx(c)

	if c.Request.Method == http.MethodPost {
		action := c.PostForm("action")

		if action == "delete" {
			pc.db.Delete(&post)
			c.Redirect(http.StatusFound, "/")
			return
		}

		editPost, _ := postFromRequest(c.Request)
		editPost.Id = post.Id

		pc.db.Save(editPost)

		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "edit-post.html", Rcx(c, Cx{
		"post": post,
	}))
}

func (pc *PostController) DeletePostHandler(c *gin.Context) {
	post := postFromCtx(c)
	pc.db.Delete(&post)

	c.Header("HX-Redirect", "/")
	c.Redirect(http.StatusOK, "/")
}

func (pc *PostController) NewPostHandler(c *gin.Context) {
	if c.Request.Method == http.MethodPost {
		post, _ := postFromRequest(c.Request)

		pc.db.Save(post)
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "new-post.html", Rcx(c, Cx{}))
}

//go:embed all:static
var assetsFS embed.FS

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

		var pages []int64

		for p := int64(1); p <= totalCount/10; p++ {
			pages = append(pages, p)
		}

		c.HTML(http.StatusOK, "index.html", Rcx(c, Cx{
			"posts":      posts,
			"years":      yearEntries,
			"yearFilter": yearInt,
			"totalCount": totalCount,
			"perPage":    10,
			"page":       page,
			"pages":      pages,
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
