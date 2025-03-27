package internal

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"image/color"
	"io"
	"mime/multipart"
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

type PostRequest struct {
	AttachmentIds string    `form:"attachmentIds" json:"attachmentIds"`
	EventTime     time.Time `form:"eventTime" json:"eventTime" time_format:"2006-01-02T15:04"`
	Body          string    `form:"body" json:"body"`
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
			var postRequest PostRequest
			c.ShouldBind(&postRequest)

			post := Post{
				EventTime: postRequest.EventTime,
				Body:      postRequest.Body,
			}

			if err := db.Save(&post).Error; err != nil {
				fmt.Println(err)
			}

			attachmentIds := strings.Split(postRequest.AttachmentIds, ",")

			for _, attachmentId := range attachmentIds {
				attachment := Attachment{
					Id: uuid.MustParse(attachmentId),
				}

				db.Model(&attachment).Updates(Attachment{
					InUse:  true,
					PostId: post.Id,
				})
			}

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

func CreateThumbnail(originalFilePath string, width, height int, thumbFilePath string) error {
	img, err := imaging.Open(originalFilePath, imaging.AutoOrientation(true))

	if err != nil {
		return err
	}

	thumbnail := imaging.Thumbnail(img, width, height, imaging.CatmullRom)

	// create a new blank image
	dst := imaging.New(width, height, color.NRGBA{})

	// paste thumbnails into the new image side by side
	dst = imaging.PasteCenter(dst, thumbnail)

	// save the combined image to file
	err = imaging.Save(dst, thumbFilePath)

	return err
}

func CalculateHash(file *multipart.FileHeader) (*string, error) {
	openedFile, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer openedFile.Close()

	// create SHA1-Hash
	hasher := sha1.New()
	if _, err := io.Copy(hasher, openedFile); err != nil {
		return nil, err
	}

	// convert hash to hex
	sha1Hash := hex.EncodeToString(hasher.Sum(nil))
	return &sha1Hash, nil
}

const thumbSize = 500

type UploadReponse struct {
	Url          string    `json:"url"`
	Href         string    `json:"href"`
	AttachmentId uuid.UUID `json:"attachmentId"`
	FileHash     string    `json:"fileHash"`
}

func UploadFileHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, _ := c.FormFile("file")
		fileHash, _ := CalculateHash(file)

		ext := filepath.Ext(file.Filename)

		fileNameBase := *fileHash
		folder := "uploads/" + fileNameBase[0:2] + "/"

		filePath := folder + fileNameBase + ext
		thumbFilePath := folder + fileNameBase + "-thumb" + ext

		// Upload the file to specific dst.
		err := c.SaveUploadedFile(file, filePath)
		if err != nil {
			return
		}

		go CreateThumbnail(filePath, thumbSize, thumbSize, thumbFilePath)

		attachment := Attachment{
			FilePath: filePath,
			FileHash: *fileHash,
		}

		db.Create(&attachment)

		uploadResponse := &UploadReponse{
			Url:          "/" + thumbFilePath,
			Href:         "/" + filePath + "?content-disposition=attachment",
			AttachmentId: attachment.Id,
			FileHash:     attachment.FileHash,
		}
		c.JSON(http.StatusOK, uploadResponse)
	}
}
