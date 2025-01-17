package internal

import (
	"embed"
	"github.com/flosch/pongo2"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"io/fs"
	"net/http"
	"os"
)

//go:embed all:templates/*
var templateFS embed.FS

type RenderOptions struct {
	TemplateDir string
	TemplateSet *pongo2.TemplateSet
	ContentType string
}

// Pongo2Render is a custom Gin template renderer using Pongo2.
type Pongo2Render struct {
	Options  *RenderOptions
	Template *pongo2.Template
	Context  pongo2.Context
}

// New creates a new Pongo2Render instance with custom Options.
func New(options RenderOptions) *Pongo2Render {
	// If TemplateSet is nil, rather than using pongo2.DefaultSet,
	// construct a new TemplateSet with the correct base directory.
	// This avoids the need to call pongo2.DefaultLoader.SetBaseDir,
	// and is necessary to support multiple Pongo2Render instances.
	if options.TemplateSet == nil {
		loader := pongo2.MustNewLocalFileSystemLoader(options.TemplateDir)
		options.TemplateSet = pongo2.NewSet(options.TemplateDir, loader)
		options.TemplateSet.Debug = gin.IsDebugging()
	}

	return &Pongo2Render{
		Options: &options,
	}
}

// Default creates a Pongo2Render instance with default options.
func DefaultPongo2(isDebug bool) *Pongo2Render {
	var content fs.FS

	if isDebug {
		content = os.DirFS("internal/templates")
	} else {
		content, _ = fs.Sub(templateFS, "templates")
	}

	templateSet := pongo2.NewSet("", &Loader{Content: content})
	templateSet.Debug = isDebug

	return New(RenderOptions{
		TemplateSet: templateSet,
		ContentType: "text/html; charset=utf-8",
	})
}

// Instance should return a new Pongo2Render struct per request and prepare
// the template by either loading it from disk or using pongo2's cache.
func (p Pongo2Render) Instance(name string, data interface{}) render.Render {
	// TemplateSet.FromCache will only cache templates if TemplateSet.Debug = true
	// This is populated from gin.Mode() in the constructor, see: New

	// this allows us to pass `nil` as data in the render methods
	if data == nil {
		data = pongo2.Context{}
	}

	return Pongo2Render{
		Template: pongo2.Must(p.Options.TemplateSet.FromCache(name)),
		Context:  data.(pongo2.Context),
		Options:  p.Options,
	}
}

// Render should render the template to the response.
func (p Pongo2Render) Render(w http.ResponseWriter) error {
	p.WriteContentType(w)
	err := p.Template.ExecuteWriter(p.Context, w)
	return err
}

// WriteContentType should add the Content-Type header to the response
// when not set yet.
func (p Pongo2Render) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = []string{p.Options.ContentType}
	}
}

type Cx = pongo2.Context

func Rcx(c *gin.Context, mm Cx) Cx {
	ctx := Cx{}

	for k, v := range c.Keys {
		ctx[k] = v
	}

	delete(ctx, sessions.DefaultKey)

	for k, v := range mm {
		ctx[k] = v
	}

	return ctx
}
