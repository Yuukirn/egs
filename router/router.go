package router

import (
	"github.com/Yuukirn/egs/security"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/jinzhu/copier"
	"github.com/mcuadros/go-defaults"
	"net/http"
	"reflect"
)

type Request struct {
	Description string
	Model       any
	Headers     openapi3.Headers
}

type Response map[string]ResponseItem

type ResponseItem struct {
	Description string
	Model       any
	Headers     openapi3.Headers
}

type Enum map[string]EnumItem

type EnumItem struct {
	Name        string
	Kind        string // `string` or `integer`
	Values      []any
	Description string
}

type Router struct {
	// middlewares
	Handlers            []gin.HandlerFunc
	Path                string
	Method              string
	Summary             string
	Description         string
	OperationID         string
	Deprecated          bool
	Exclude             bool
	RequestContentType  string
	ResponseContentType string
	Tags                []string

	// handler
	API        gin.HandlerFunc
	Model      any
	Securities []security.Security
	Response   Response
	Request    Request
	Enum       Enum
}

type Option func(router *Router)

func Req(request Request) Option {
	return func(router *Router) {
		router.Request = request
	}
}

func Resp(response Response) Option {
	return func(router *Router) {
		router.Response = response
	}
}

func Security(securities ...security.Security) Option {
	return func(router *Router) {
		router.Securities = append(router.Securities, securities...)
	}
}

func Enums(enums Enum) Option {
	return func(router *Router) {
		router.Enum = enums
	}
}

func Tags(tags ...string) Option {
	return func(router *Router) {
		router.Tags = append(router.Tags, tags...)
	}
}

func Summary(summary string) Option {
	return func(router *Router) {
		router.Summary = summary
	}
}

func Desc(desc string) Option {
	return func(router *Router) {
		router.Description = desc
	}
}

func OperationID(ID string) Option {
	return func(router *Router) {
		router.OperationID = ID
	}
}

func Deprecated() Option {
	return func(router *Router) {
		router.Deprecated = true
	}
}

func Exclude() Option {
	return func(router *Router) {
		router.Exclude = true
	}
}

func NewRouterX(f gin.HandlerFunc, options ...Option) *Router {
	r := &Router{
		Handlers: make([]gin.HandlerFunc, 0),
		API:      f,
		Response: make(Response),
	}

	for _, option := range options {
		option(r)
	}

	return r
}

func (router *Router) GetHandlers() []gin.HandlerFunc {
	var handlers []gin.HandlerFunc
	for _, handler := range router.Handlers {
		handlers = append(handlers, handler)
	}
	handlers = append(handlers, router.API)
	return handlers
}

func NewRouter[T any, F func(c *gin.Context, req T)](f F, options ...Option) *Router {
	var req T
	bindMiddleware := bindRequest(&req)
	router := &Router{
		Response: make(Response),
		API: func(c *gin.Context) {
			f(c, req)
		},
		Model: req,
	}

	for _, option := range options {
		option(router)
	}

	router.Handlers = append(router.Handlers, bindMiddleware)
	return router
}

func bindRequest(req any) gin.HandlerFunc {
	return func(c *gin.Context) {
		model := reflect.New(reflect.TypeOf(req).Elem()).Interface()
		if err := c.ShouldBindHeader(model); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
		}
		if err := c.ShouldBindQuery(model); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
		}
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut {
			switch c.Request.Header.Get("Content-Type") {
			case binding.MIMEMultipartPOSTForm:
				if err := c.ShouldBindWith(model, binding.FormMultipart); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
				}
			case binding.MIMEJSON:
				if err := c.ShouldBindJSON(model); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
				}
			case binding.MIMEXML:
				if err := c.ShouldBindXML(model); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
				}
			case binding.MIMEPOSTForm:
				if err := c.ShouldBindWith(model, binding.Form); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
				}
			case binding.MIMEYAML:
				if err := c.ShouldBindYAML(model); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
				}
			case binding.MIMEPROTOBUF:
				if err := c.ShouldBindWith(model, binding.ProtoBuf); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
				}
			case binding.MIMEMSGPACK:
				if err := c.ShouldBindWith(model, binding.MsgPack); err != nil {
					c.AbortWithStatus(http.StatusBadRequest)
				}
			}
		}
		if err := c.ShouldBindUri(model); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
		}

		defaults.SetDefaults(model)
		if err := validator.New().Struct(model); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
		}
		if err := copier.Copy(req, model); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
		}
		c.Next()
	}
}
