package internal

import (
	"embed"
	"github.com/flosch/pongo2"
	"github.com/russross/blackfriday/v2"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed all:templates/*
var templateFS embed.FS
var templateSet *pongo2.TemplateSet

func markdownFilter(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	extensions := blackfriday.CommonExtensions | blackfriday.HardLineBreak

	return pongo2.AsSafeValue(string(blackfriday.Run([]byte(in.String()),
		blackfriday.WithExtensions(extensions)))), nil
}

func init() {
	pongo2.RegisterFilter("markdown", markdownFilter)

	content := os.DirFS("internal")
	templateSet = pongo2.NewSet("", &Loader{Content: content})
	templateSet.Debug = true
}

func Render(w http.ResponseWriter, name string, data map[string]any) {
	ctx := pongo2.Context{}

	if data != nil {
		for k, v := range data {
			ctx[k] = v
		}
	}

	tpl, err := templateSet.FromFile(filepath.Join("templates", name))

	if err != nil {
		panic(err)
	}

	tpl.ExecuteWriter(ctx, w)
}
