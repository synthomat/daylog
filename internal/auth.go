package internal

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"regexp"
)

func AuthMiddleware() gin.HandlerFunc {
	rex, err := regexp.Compile("^/(static|login|logout)")

	if err != nil {
		panic(err)
	}

	return func(c *gin.Context) {
		session := sessions.Default(c)

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

	if c.Request.Method == http.MethodPost {

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
