package security

import (
	"github.com/getkin/kin-openapi/openapi3"
)

type Bearer struct {
	AuthName string
}

func (b *Bearer) Name() string {
	return b.AuthName
}

func (b *Bearer) Schema() *openapi3.SecurityScheme {
	return &openapi3.SecurityScheme{
		Type:   "http",
		Scheme: "bearer",
	}
}
