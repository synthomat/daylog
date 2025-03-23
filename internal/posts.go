package internal

import (
	"errors"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"image"
	"image/color"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

const DeletePostAction = "delete"

func EditPostHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		post := c.MustGet("post").(Post)

		if c.Request.Method == http.MethodPost {
			action := c.PostForm("action")

			if action == DeletePostAction {
				db.Delete(&post)
				c.Redirect(http.StatusFound, "/")
				return
			}

			editPost, _ := postFromRequest(c.Request)
			editPost.Id = post.Id

			db.Save(editPost)

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

func NewPostHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			post, _ := postFromRequest(c.Request)

			db.Save(post)
			c.Redirect(http.StatusFound, "/")
			return
		}

		c.HTML(http.StatusOK, "new-post.html", Rcx(c, Cx{}))
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

		query := QueryByFilter(db, filter)

		var posts []Post
		var totalCount int64

		query.Model(Post{})

		if !authedForArchive {
			query.Where("(unixepoch() - unixepoch(event_time)) < 86400 * ?", maxArchivedDays)
		}
		query.Count(&totalCount)

		query.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&posts)

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

func CreateThumbnail(originalFilePath string, width, height int, thumbFilePath string) {
	img, err := imaging.Open(originalFilePath, imaging.AutoOrientation(true))
	if err != nil {
		panic(err)
	}
	var thumbnail image.Image
	thumbnail = imaging.Thumbnail(img, 300, 300, imaging.CatmullRom)

	// create a new blank image
	dst := imaging.New(width, height, color.NRGBA{})

	// paste thumbnails into the new image side by side
	dst = imaging.Paste(dst, thumbnail, image.Pt(0, 0))

	// save the combined image to file
	err = imaging.Save(dst, thumbFilePath)
}

func UploadFileHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, _ := c.FormFile("file")

		ext := filepath.Ext(file.Filename)

		fileNameBase := uuid.New().String()
		folder := "uploads/" + fileNameBase[0:2] + "/"

		filePath := folder + fileNameBase + ext
		thumbFilePath := folder + fileNameBase + "-thumb" + ext

		// Upload the file to specific dst.
		c.SaveUploadedFile(file, filePath)

		go CreateThumbnail(filePath, 400, 400, thumbFilePath)

		c.JSON(http.StatusOK, gin.H{
			"url":  "/" + thumbFilePath,
			"href": "/" + filePath + "?content-disposition=attachment",
		})
	}
}
