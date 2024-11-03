package internal

import (
	"context"
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
	"regexp"
	"strconv"
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

			post := Post{
				BaseModel: BaseModel{Id: postUuid},
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

func postFromReq(r *http.Request) Post {
	return r.Context().Value("post").(Post)
}

func AuthMiddleware(store sessions.Store) func(next http.Handler) http.Handler {
	rex, err := regexp.Compile("^/(static|login|logout)")

	if err != nil {
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := store.Get(r, sessionName)

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
var assetsFS embed.FS

func Run(config Config) {
	var store = sessions.NewCookieStore([]byte(config.SessionKey))
	store.Options.Secure = false
	store.Options.SameSite = http.SameSiteLaxMode

	db, err := NewDB(config.DBFileName)

	if err != nil {
		log.Fatalf("Could not open database: %s\n", err.Error())
	}

	/*
		err = os.MkdirAll("uploads", os.ModePerm)

		if err != nil {
			fmt.Println("Could not create upload folder")
			return
		}
	*/

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(AuthMiddleware(store))

	// Use the file system to serve static files
	assetsFS, _ := fs.Sub(assetsFS, "static")
	sfs := http.FileServer(http.FS(assetsFS))
	r.Handle("/static/*", http.StripPrefix("/static/", sfs))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var posts []Post
		db.Order("event_time desc").Find(&posts)

		Render(w, "index.html", map[string]any{
			"posts": posts,
		})
	})

	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()

			password := r.FormValue("password")
			if password == config.AuthSecret {
				session, _ := store.Get(r, sessionName)
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
		Render(w, "login.html", map[string]any{})
	})

	r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, sessionName)
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

		Render(w, "new-post.html", nil)
	})

	r.With(InjectPost(db)).
		Route("/posts/{pid}", func(r chi.Router) {
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

				Render(w, "edit-post.html", map[string]any{
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

	err = http.ListenAndServe("0.0.0.0:"+strconv.Itoa(config.Port), r)

	if err != nil {
		log.Fatalf("Could not start application: %s\n", err.Error())
		return
	}
}
