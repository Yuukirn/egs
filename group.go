package egs

import (
	"github.com/Yuukirn/egs/router"
	"github.com/Yuukirn/egs/security"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Group struct {
	Egs         *Egs
	Path        string
	Tags        []string
	RouterGroup *gin.RouterGroup

	// middlewares
	Handlers   []gin.HandlerFunc
	Securities []security.Security
}

type GroupOption func(group *Group)

func Handlers(handlers ...gin.HandlerFunc) GroupOption {
	return func(g *Group) {
		g.Handlers = append(g.Handlers, handlers...)
	}
}

func Tags(tags string) GroupOption {
	return func(g *Group) {
		g.Tags = append(g.Tags, tags)
	}
}

func Security(securities ...security.Security) GroupOption {
	return func(g *Group) {
		g.Securities = append(g.Securities, securities...)
	}
}

func (g *Group) Use(middleware ...gin.HandlerFunc) gin.IRoutes {
	return g.RouterGroup.Use(middleware...)
}

func (g *Group) handle(method, path string, r *router.Router) {
	r.Handlers = append(r.Handlers, g.Handlers...)
	r.Tags = append(r.Tags, g.Tags...)
	r.Securities = append(r.Securities, g.Securities...)
	g.Egs.handle(g.Path+path, method, r)
}

func (g *Group) GET(path string, r *router.Router) {
	g.handle(http.MethodGet, path, r)
}

func (g *Group) POST(path string, r *router.Router) {
	g.handle(http.MethodPost, path, r)
}

func (g *Group) HEAD(path string, r *router.Router) {
	g.handle(http.MethodHead, path, r)
}

func (g *Group) PATCH(path string, r *router.Router) {
	g.handle(http.MethodPatch, path, r)
}

func (g *Group) DELETE(path string, r *router.Router) {
	g.handle(http.MethodDelete, path, r)
}

func (g *Group) OPTIONS(path string, r *router.Router) {
	g.handle(http.MethodOptions, path, r)
}

func (g *Group) PUT(path string, r *router.Router) {
	g.handle(http.MethodPut, path, r)
}

func (g *Group) Group(path string, options ...GroupOption) *Group {
	group := &Group{
		Egs:         g.Egs,
		Path:        g.Path + path,
		Tags:        g.Tags,
		RouterGroup: g.RouterGroup.Group(path),
		Handlers:    g.Handlers,
		Securities:  g.Securities,
	}

	for _, option := range options {
		option(group)
	}

	return group
}
