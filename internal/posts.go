package internal

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
	"time"
)

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
