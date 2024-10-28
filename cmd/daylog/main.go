package main

import (
	"context"
	"daylog/internal"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
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

func main() {
	db, _ := internal.NewDB("daylog.db")

	r := chi.NewRouter()

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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var posts []internal.Post
		db.Order("event_time desc").Find(&posts)

		internal.Render(w, "index.html", map[string]any{
			"posts": posts,
		})
	})

	r.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
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

			if r.Method == "POST" {
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
