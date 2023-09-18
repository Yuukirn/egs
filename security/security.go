package security

import (
	"github.com/getkin/kin-openapi/openapi3"
)

// Security TODO add oauth and oicd
type Security interface {
	Schema() *openapi3.SecurityScheme
	Name() string
}
