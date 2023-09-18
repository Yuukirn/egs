package security

import (
	"github.com/getkin/kin-openapi/openapi3"
)

type ApiKey struct {
	AuthName string
	name     string
	in       string // header query cookie
}

func (a *ApiKey) Name() string {
	return a.AuthName
}

func (a *ApiKey) Schema() *openapi3.SecurityScheme {
	return &openapi3.SecurityScheme{
		Type: "apiKey",
		In:   a.in,
		Name: a.name,
	}
}
