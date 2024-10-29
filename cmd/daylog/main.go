package main

import (
	"context"
	"daylog/internal"
	"embed"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"gorm.io/gorm"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
)

// Parses the request and returns a Post struct
func postFromRequest(r *http.Request) (*internal.Post, error) {
	eventTime, _ := time.Parse("2006-01-02T15:04", r.FormValue("event_time"))

	title := r.FormValue("title")

	post := internal.Post{
		EventTime: eventTime,
		Title:     &title,
		Body:      r.FormValue("body"),
	}

	return &post, nil
}

type Server struct {
	Db     *gorm.DB
	Router chi.Router
}

func (s *Server) Run(router *chi.Mux) {
	http.ListenAndServe("0.0.0.0:3002", router)
}

func InjectPost(db *gorm.DB) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			postId := chi.URLParam(r, "pid")
			postUuid := uuid.MustParse(postId)

			post := internal.Post{
				BaseModel: internal.BaseModel{Id: postUuid},
			}

			err := db.First(&post).Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				w.Write([]byte("Post not found"))
				w.WriteHeader(http.StatusNotFound)
				return
			}

			ctx := context.WithValue(r.Context(), "post", post)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func postFromReq(r *http.Request) internal.Post {
	return r.Context().Value("post").(internal.Post)
}

func AuthMiddleware(store sessions.Store) func(next http.Handler) http.Handler {
	rex, err := regexp.Compile("^/(static|login|logout)")

	if err != nil {
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := store.Get(r, "session-name")

			if err != nil {
				fmt.Print(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			if rex.Match([]byte(r.URL.Path)) {
				next.ServeHTTP(w, r)
				return
			}

			if session.Values["authenticated"] == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

//go:embed all:static
var assets embed.FS

func Assets() (fs.FS, error) {
	return fs.Sub(assets, "static")
}

func main() {
	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		log.Fatal("No SESSION_KEY environment variable set")
		return
	}

	authSecret := os.Getenv("AUTH_SECRET")

	if authSecret == "" {
		log.Fatal("No AUTH_SECRET environment variable set")
	}

	databaseFile := "daylog.db"

	var store = sessions.NewCookieStore([]byte(sessionKey))
	store.Options.Secure = false
	store.Options.SameSite = http.SameSiteLaxMode

	db, _ := internal.NewDB(databaseFile)

	r := chi.NewRouter()

	assets, _ := Assets()

	/*
		err = os.MkdirAll("uploads", os.ModePerm)

		if err != nil {
			fmt.Println("Could not create upload folder")
			return
		}
	*/
	server := Server{
		Db: db,
	}

	r.Use(middleware.Logger)
	r.Use(AuthMiddleware(store))

	// Use the file system to serve static files
	sfs := http.FileServer(http.FS(assets))
	r.Handle("/static/*", http.StripPrefix("/static/", sfs))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var posts []internal.Post
		db.Order("event_time desc").Find(&posts)

		internal.Render(w, "index.html", map[string]any{
			"posts": posts,
		})
	})

	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()

			password := r.FormValue("password")
			if password == authSecret {
				session, _ := store.Get(r, "session-name")
				session.Values["authenticated"] = true

				err := session.Save(r, w)
				if err != nil {
					fmt.Println(err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}

		}
		internal.Render(w, "login.html", map[string]any{})
	})

	r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session-name")
		delete(session.Values, "authenticated")
		session.Options.MaxAge = -1
		session.Save(r, w)
		w.Header().Add("HX-Redirect", "/login")
		http.Redirect(w, r, "/login", http.StatusOK)
		return
	})

	r.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()

			post, _ := postFromRequest(r)

			db.Save(post)
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		internal.Render(w, "new-post.html", nil)
	})

	r.With(InjectPost(db)).Route("/posts/{pid}", func(r chi.Router) {
		r.HandleFunc("/edit", func(w http.ResponseWriter, r *http.Request) {
			post := postFromReq(r)

			if r.Method == http.MethodPost {
				r.ParseForm()

				action := r.FormValue("action")

				if action == "delete" {
					db.Delete(&post)
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}

				editPost, _ := postFromRequest(r)
				editPost.Id = post.Id

				db.Save(editPost)
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}

			internal.Render(w, "edit-post.html", map[string]any{
				"post": post,
			})
		})

		r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
			post := postFromReq(r)
			db.Delete(&post)

			w.Header().Add("HX-Redirect", "/")
			w.WriteHeader(http.StatusNoContent)
		})
	})

	server.Run(r)
}
