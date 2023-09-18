package security

import (
	"github.com/getkin/kin-openapi/openapi3"
)

type Basic struct {
	AuthName string
}

func (b *Basic) Name() string {
	return b.AuthName
}

func (b *Basic) Schema() *openapi3.SecurityScheme {
	return &openapi3.SecurityScheme{
		Type:   "http",
		Scheme: "basic",
	}
}
