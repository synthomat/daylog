package internal

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InjectPostMiddleware is a middleware that injects the Post struct into the request context from the post-id in the URL
func InjectPostMiddleware(db *gorm.DB) gin.HandlerFunc {
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

type PostRequest struct {
	EventTime     time.Time `form:"eventTime" time_format:"2006-01-02"`
	Body          string    `form:"body" binding:"required"`
	AttachmentIds string    `form:"attachmentIds"`
}

// Parses the request and returns a Post struct
func postFromRequest(r *http.Request) (*Post, error) {
	eventTime, _ := time.Parse("2006-01-02T15:04", r.FormValue("eventTime"))

	title := r.FormValue("title")

	post := Post{
		EventTime: eventTime,
		Title:     &title,
		Body:      r.FormValue("body"),
	}
	fmt.Println(r.FormValue("attachmentIds"))

	return &post, nil
}

func NewPostHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		postRequest := PostRequest{EventTime: time.Now()}

		if c.Request.Method == http.MethodPost {
			c.Bind(&postRequest)

			var post Post
			PostRequestToModel(postRequest, &post)
			db.Save(post)

			err := db.Save(&post).Error
			if err != nil {
				fmt.Println(err)
			}

			c.Redirect(http.StatusFound, "/")
			return
		}

		c.HTML(http.StatusOK, "new-post.html", Rcx(c, Cx{"post": postRequest}))
	}
}

const DeletePostAction = "delete"

func PostRequestToModel(req PostRequest, model *Post) {
	model.Body = req.Body
	model.EventTime = req.EventTime
}

func EditPostHandler(db *gorm.DB) gin.HandlerFunc {
	//attachmentManager := &AttachmentManager{db: db}

	return func(c *gin.Context) {
		// get Post object from middleware
		post := c.MustGet("post").(Post)

		if c.Request.Method == http.MethodPost {
			action := c.PostForm("action")

			if action == DeletePostAction {
				db.Delete(&post)
				c.Redirect(http.StatusFound, "/")
				return
			}

			var postRequest PostRequest
			c.ShouldBind(&postRequest)

			PostRequestToModel(postRequest, &post)
			db.Save(post)

			c.Redirect(http.StatusFound, "/")
			return
		}

		c.HTML(http.StatusOK, "edit-post.html", Rcx(c, Cx{
			"post": post,
		}))
	}
}

func DeletePostHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		post := c.MustGet("post").(Post)
		db.Delete(&post)

		c.Header("HX-Redirect", "/")
		c.Redirect(http.StatusOK, "/")
	}
}

type PaginationLink struct {
	Ord    int64
	Params string
}

type QueryFilter struct {
	Page     int  `form:"p"`
	PageSize int  `form:"s"`
	Year     *int `form:"y"`

	Search string `form:"q"`
}

func ParseFilter(c *gin.Context) (*QueryFilter, error) {
	var filter QueryFilter
	c.BindQuery(&filter)

	if filter.Page < 1 {
		filter.Page = 1
	}

	if filter.PageSize < 1 {
		filter.PageSize = 20
	}

	return &filter, nil
}

func QueryByFilter(db *gorm.DB, filter *QueryFilter) *gorm.DB {
	query := db.Order("event_time desc")

	// "full-text search"
	if search := filter.Search; search != "" {
		query.Where("lower(body) like ?", "%"+strings.ToLower(search)+"%")
	}

	if year := filter.Year; year != nil {
		query.Where("strftime('%Y', event_time) = ?", strconv.Itoa(*year))
	}

	return query
}

type PostYears struct {
	Year  string
	Count int
}

const maxArchivedDays = 7

func IndexHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authedForArchive := c.GetBool("authedForArchive")

		filter, _ := ParseFilter(c)
		var posts []Post

		query := QueryByFilter(db.Table("posts"), filter)

		var totalCount int64

		//query.Model(&posts)

		if !authedForArchive {
			query.Where("(unixepoch() - unixepoch(event_time)) < 86400 * ?", maxArchivedDays)
		}
		query.Count(&totalCount)

		err := query.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&posts).Error
		if err != nil {
			fmt.Println(err)
		}

		var yearEntries []PostYears
		db.Raw(
			"SELECT DISTINCT strftime('%Y', event_time) as year, count(*) as count\n" +
				"FROM posts\n" +
				"GROUP BY year\n" +
				"ORDER BY year DESC").Scan(&yearEntries)

		var paginationLinks []PaginationLink

		q := c.Request.URL.Query()

		for p := int64(1); p <= totalCount/int64(filter.PageSize); p++ {
			q.Set("p", strconv.FormatInt(p, 10))

			pl := PaginationLink{
				Ord:    p,
				Params: q.Encode(),
			}

			paginationLinks = append(paginationLinks, pl)
		}

		c.HTML(http.StatusOK, "index.html", Rcx(c, Cx{
			"posts":       posts,
			"searchQuery": filter.Search,
			"years":       yearEntries,
			"totalCount":  totalCount,
			"page":        filter.Page,
			"pagination":  paginationLinks,
			"reauth":      !authedForArchive,
		}))
	}
}
