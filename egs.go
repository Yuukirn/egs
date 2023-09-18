package egs

import (
	"embed"
	"encoding/json"
	"github.com/Yuukirn/egs/router"
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
	"strings"
)

//go:embed templates/*
var templates embed.FS

type RouterMap map[*gin.RouterGroup]map[string]map[string]*router.Router

type Egs struct {
	*gin.Engine

	// Swagger is used to construct swagger json
	Swagger *Swagger

	Routers RouterMap
}

func New(swagger *Swagger) *Egs {
	engine := gin.Default()
	egs := &Egs{
		Engine:  engine,
		Swagger: swagger,
		Routers: make(RouterMap),
	}
	egs.Routers[&egs.RouterGroup] = make(map[string]map[string]*router.Router)

	egs.SetHTMLTemplate(template.Must(template.ParseFS(templates, "templates/*.html")))

	// set swagger router
	if swagger != nil {
		swagger.Routers = egs.Routers
	}

	return egs
}

func (e *Egs) Use(middlewares ...gin.HandlerFunc) gin.IRoutes {
	return e.Engine.Use(middlewares...)
}

func (e *Egs) Group(path string, options ...GroupOption) *Group {
	group := &Group{
		Egs:         e,
		RouterGroup: e.Engine.Group(path),
		Path:        path,
	}

	for _, option := range options {
		option(group)
	}

	return group
}

func (e *Egs) handle(path, method string, r *router.Router) {
	r.Method = method
	r.Path = path

	if e.Routers[&e.RouterGroup][path] == nil {
		e.Routers[&e.RouterGroup][path] = make(map[string]*router.Router)
	}

	e.Routers[&e.RouterGroup][path][method] = r
}

func (e *Egs) GET(path string, r *router.Router) {
	e.handle(path, http.MethodGet, r)
}

func (e *Egs) POST(path string, r *router.Router) {
	e.handle(path, http.MethodPost, r)
}

func (e *Egs) HEAD(path string, r *router.Router) {
	e.handle(path, http.MethodHead, r)
}

func (e *Egs) PUT(path string, r *router.Router) {
	e.handle(path, http.MethodPut, r)
}

func (e *Egs) DELETE(path string, r *router.Router) {
	e.handle(path, http.MethodDelete, r)
}

func (e *Egs) PATCH(path string, r *router.Router) {
	e.handle(path, http.MethodPatch, r)
}

func (e *Egs) OPTIONS(path string, r *router.Router) {
	e.handle(path, http.MethodOptions, r)
}

func (e *Egs) init() {
	e.initRouters()
	if e.Swagger == nil {
		return
	}
	gin.DisableBindValidation()
	e.Engine.GET(e.Swagger.OpenAPIUrl, func(c *gin.Context) {
		if strings.HasSuffix(e.Swagger.OpenAPIUrl, ".yml") || strings.HasSuffix(e.Swagger.OpenAPIUrl, ".yaml") {
			yaml, err := e.Swagger.MarshalYaml()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.String(http.StatusOK, string(yaml))
		} else {
			c.JSON(http.StatusOK, e.Swagger)
		}
	})

	e.Engine.GET(e.Swagger.DocsUrl, func(c *gin.Context) {
		options := "{}"
		if e.Swagger.SwaggerOptions != nil {
			bytes, err := json.Marshal(e.Swagger.SwaggerOptions)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			options = string(bytes)
		}
		c.HTML(http.StatusOK, "swagger.html", gin.H{
			"openapi_url":     e.Swagger.OpenAPIUrl,
			"title":           e.Swagger.Title,
			"swagger_options": options,
		})
	})

	e.Engine.GET(e.Swagger.RedocUrl, func(c *gin.Context) {
		options := "{}"
		if e.Swagger.RedocOptions != nil {
			bytes, err := json.Marshal(e.Swagger.RedocOptions)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			options = string(bytes)
		}
		c.HTML(http.StatusOK, "redoc.html", gin.H{
			"openapi_url":   e.Swagger.OpenAPIUrl,
			"title":         e.Swagger.Title,
			"redoc_options": options,
		})
	})

	e.Swagger.BuildOpenAPI()
}

func (e *Egs) initRouters() {
	for group, routers := range e.Routers {
		for path, m := range routers {
			for method, r := range m {
				handlers := r.GetHandlers()
				switch method {
				case http.MethodGet:
					group.GET(path, handlers...)
				case http.MethodPost:
					group.POST(path, handlers...)
				case http.MethodPut:
					group.PUT(path, handlers...)
				case http.MethodDelete:
					group.DELETE(path, handlers...)
				case http.MethodOptions:
					group.OPTIONS(path, handlers...)
				case http.MethodHead:
					group.OPTIONS(path, handlers...)
				default:
					group.Any(path, handlers...)
				}
			}
		}
	}
}

func (e *Egs) Run(addr ...string) error {
	e.init()
	return e.Engine.Run(addr...)
}
