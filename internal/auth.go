package internal

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"regexp"
	"time"
)

func AuthMiddleware() gin.HandlerFunc {
	rex, _ := regexp.Compile("^/(static|login|logout)")

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

		createdAt := session.Get("createdAt")

		authedForArchive := (time.Now().Unix() - createdAt.(int64)) < 60*30
		c.Set("authedForArchive", authedForArchive)

		c.Set("authenticated", true)
		if err := session.Save(); err != nil {
			_ = fmt.Errorf("error saving session: %v", err)
		}

		c.Next()
	}
}

var devices = map[string]string{}

func LoginHandler(db *gorm.DB, config Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		if c.Request.Method == http.MethodPost {
			password := c.PostForm("password")

			if password != config.AuthSecret {
				c.Redirect(http.StatusSeeOther, "/login")
				return
			}

			session.Set("authenticated", true)
			session.Set("createdAt", time.Now().Unix())
			err := session.Save()

			if err != nil {
				fmt.Println(err.Error())
				c.AbortWithError(http.StatusInternalServerError, err)
			}

			/*
				userAgent := c.Request.UserAgent()
				fmt.Println("User Agent:", userAgent)

				deviceId, err := c.Cookie("dl-device")
				_, deviceFound := devices[deviceId]

				if err != nil || !deviceFound {
					slog.Info("New device found!")
					twoMonthsInSeconds := 60 * 60 * 24 * 31 * 2
					deviceId = uuid.New().String()
					c.SetCookie("dl-device", deviceId, twoMonthsInSeconds, "/", "", false, true)
					devices[deviceId] = userAgent
				}
			*/

			c.Redirect(http.StatusFound, "/")
			return

		}

		c.HTML(http.StatusOK, "login.html", nil)
	}
}

func ClearSession(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
}

func LogoutHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ClearSession(c)

		c.Header("HX-Redirect", "/")
		c.Header("Location", "/")
		c.Writer.WriteHeader(http.StatusOK)
		//c.String(http.StatusOK, "")
	}
}
