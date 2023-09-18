package egs

import (
	"encoding/json"
	"github.com/Yuukirn/egs/router"
	"github.com/Yuukirn/egs/security"
	"github.com/fatih/structtag"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin/binding"
	"github.com/invopop/yaml"
	"mime/multipart"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const (
	DEFAULT     = "default"
	BINDING     = "binding"
	DESCRIPTION = "description"
	QUERY       = "query"
	FORM        = "form"
	URI         = "uri"
	HEADER      = "header"
	COOKIE      = "cookie"
	JSON        = "json"
)

type Swagger struct {
	// metadata
	Title          string
	Description    string
	TermsOfService string
	Contact        *openapi3.Contact
	License        *openapi3.License
	Version        string

	DocsUrl    string
	OpenAPIUrl string
	RedocUrl   string
	OpenAPI    *openapi3.T

	Servers openapi3.Servers

	Routers RouterMap

	SwaggerOptions map[string]any
	RedocOptions   map[string]any
}

func NewSwagger(title, desc, version string) *Swagger {
	return &Swagger{
		Title:          title,
		Description:    desc,
		Version:        version,
		DocsUrl:        "/docs",
		RedocUrl:       "/redoc",
		OpenAPIUrl:     "/openapi.json",
		SwaggerOptions: make(map[string]any),
		RedocOptions:   make(map[string]any),
		Routers:        make(RouterMap),
	}
}

func (swagger *Swagger) MarshalJSON() ([]byte, error) {
	return swagger.OpenAPI.MarshalJSON()
}

func (swagger *Swagger) MarshalYaml() ([]byte, error) {
	bytes, err := swagger.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var data any
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(data)
}

func (swagger *Swagger) BuildOpenAPI() {
	components := &openapi3.Components{}
	components.SecuritySchemes = openapi3.SecuritySchemes{}
	swagger.OpenAPI = &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:          swagger.Title,
			Description:    swagger.Description,
			TermsOfService: swagger.TermsOfService,
			Contact:        swagger.Contact,
			License:        swagger.License,
			Version:        swagger.Version,
		},
		Servers:    swagger.Servers,
		Components: components,
	}
	swagger.buildPath()
}

func (swagger *Swagger) buildPath() {
	paths := make(openapi3.Paths)
	for _, routers := range swagger.Routers {
		for path, m := range routers {
			pathItem := &openapi3.PathItem{}
			for method, r := range m {
				if r.Exclude {
					continue
				}

				swagger.getComponentByModel(r.Request.Model, true)
				for _, resp := range r.Response {
					swagger.getComponentByModel(resp.Model, false)
				}
				swagger.getEnumComponent(r.Enum)

				operation := &openapi3.Operation{
					Tags:        r.Tags,
					Summary:     r.Summary,
					Description: r.Description,
					OperationID: r.OperationID,
					Responses:   swagger.getResponsesRef(r.Response, r.RequestContentType),
					Parameters:  swagger.getParametersByModel(r.Model),
					Deprecated:  r.Deprecated,
					Security:    swagger.getSecurity(r.Securities),
				}

				var requestBody *openapi3.RequestBodyRef
				reqType := reflect.TypeOf(r.Request.Model)
				if reqType != nil {
					requestType := reflect.TypeOf(r.Request.Model)
					if requestType.Kind() == reflect.Ptr {
						requestType = requestType.Elem()
					}
					requestBody = swagger.getRequestBodyRef(requestType.Name(), r.RequestContentType)
				}

				switch method {
				case http.MethodGet:
					pathItem.Get = operation
				case http.MethodPost:
					pathItem.Post = operation
					operation.RequestBody = requestBody
				case http.MethodDelete:
					pathItem.Delete = operation
				case http.MethodPut:
					pathItem.Put = operation
					operation.RequestBody = requestBody
				case http.MethodPatch:
					pathItem.Patch = operation
				case http.MethodHead:
					pathItem.Head = operation
				case http.MethodOptions:
					pathItem.Options = operation
				case http.MethodConnect:
					pathItem.Connect = operation
				case http.MethodTrace:
					pathItem.Trace = operation
				}
			}
			paths[swagger.fixPath(path)] = pathItem
		}
	}
	swagger.OpenAPI.Paths = paths
}

func (swagger *Swagger) getResponsesRef(response router.Response, contentType string) openapi3.Responses {
	ret := openapi3.NewResponses()
	for k, v := range response {
		type_ := reflect.TypeOf(v.Model)
		if type_ == nil {
			continue
		}
		if type_.Kind() == reflect.Ptr {
			type_ = type_.Elem()
		}

		schemaRef := openapi3.NewSchemaRef(generateRefName(removePackageName(type_.Name())), nil)

		var content = make(openapi3.Content)
		if contentType == "" {
			contentType = binding.MIMEJSON
		}
		content[contentType] = openapi3.NewMediaType().WithSchemaRef(schemaRef)

		description := v.Description
		ret[k] = &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: &description,
				Content:     content,
				Headers:     v.Headers,
			},
		}
	}

	return ret
}

func (swagger *Swagger) getRequestBodyRef(name, contentType string) *openapi3.RequestBodyRef {
	body := &openapi3.RequestBodyRef{
		Value: openapi3.NewRequestBody(),
	}
	body.Value.Required = true
	if contentType == "" {
		contentType = binding.MIMEJSON
	}

	schemaRef := openapi3.NewSchemaRef(generateRefName(removePackageName(name)), nil)
	body.Value.Content = openapi3.NewContent()
	body.Value.Content[contentType] = openapi3.NewMediaType().WithSchemaRef(schemaRef)
	return body
}

func (swagger *Swagger) getComponentByModel(model any, isRequest bool) {
	type_ := reflect.TypeOf(model)
	if type_ == nil {
		return
	}
	value_ := reflect.ValueOf(model)

	// dereference
	if type_.Kind() == reflect.Ptr {
		type_ = type_.Elem()
	}
	if value_.Kind() == reflect.Ptr {
		value_ = value_.Elem()
	}

	// openapi3.Schemas k -> struct name = title -> struct name
	// get struct name from request.SchemaName
	schemaRef := &openapi3.SchemaRef{}
	schemaRef.Value = openapi3.NewObjectSchema()

	// schemaRef is the outer field
	// if it is a struct, handle its fields
	if type_.Kind() == reflect.Struct {
		for i := 0; i < type_.NumField(); i++ {
			field := type_.Field(i)
			value := value_.Field(i)
			tags, err := structtag.Parse(string(field.Tag))
			if err != nil {
				panic(err)
			}
			formTag, err := tags.Get(FORM)
			if err != nil && isRequest {
				continue
			}

			if value.Kind() == reflect.Ptr {
				value = value.Elem()
			}

			var fieldName = field.Name
			if isRequest {
				fieldName = formTag.Name
			} else if jsonTag, err := tags.Get(JSON); err == nil && jsonTag != nil {
				fieldName = jsonTag.Name
			}

			bindingTag, err := tags.Get(BINDING)
			if err == nil {
				if bindingTag.Name == "required" {
					schemaRef.Value.Required = append(schemaRef.Value.Required, fieldName)
				}
			}

			enumTag := field.Tag.Get("enum")
			if enumTag != "" {
				swagger.getEnumComponentByTag(fieldName, enumTag)
			}

			if field.Type.Kind() == reflect.Struct {
				if field.Type.Name() == "Time" {
					fieldSchema := swagger.getSchemaByValue(value.Interface())
					fieldSchema.Format = "date-time"
					fieldSchema.Type = openapi3.TypeString

					schemaRef.Value.Properties[fieldName] = openapi3.NewSchemaRef("", fieldSchema)
					continue
				} else if !swagger.checkSchemaExist(field.Type.Name()) {
					swagger.getComponentByModel(reflect.New(field.Type).Elem().Interface(), isRequest)
				}
				//schemaRef.Ref = generateRefName(field.Type.Name())
				fieldSchemaRef := openapi3.NewSchemaRef(generateRefName(field.Type.Name()), nil)
				schemaRef.Value.Properties[fieldName] = fieldSchemaRef
			} else if field.Type.Kind() == reflect.Slice {
				// check if type.Elem() if built-in type
				var fieldSchema = swagger.getSchemaByValue(value.Interface())
				if !isBuiltinType(field.Type.Elem()) {
					if !swagger.checkSchemaExist(field.Type.Elem().Name()) {
						swagger.getComponentByModel(reflect.New(field.Type.Elem()).Elem().Interface(), isRequest)
					}
					fieldSchemaRef := openapi3.NewSchemaRef(generateRefName(field.Type.Elem().Name()), nil)
					fieldSchema.Items = fieldSchemaRef
				} else {
					descriptionTag, err := tags.Get(DESCRIPTION)
					if err == nil {
						fieldSchema.Description = descriptionTag.Name
					}

					defaultTag, err := tags.Get(DEFAULT)
					if err == nil {
						fieldSchema.Default = defaultTag.Name
					}
				}

				schemaRef.Value.Properties[fieldName] = openapi3.NewSchemaRef("", fieldSchema)
			} else if field.Type.Kind() == reflect.Map {
				// To define a dictionary, use type: object and use the additionalProperties
				// keyword to specify the type of values in key/value pairs.
				// the keys must be string

				// get the value type
				mapValueType := field.Type.Elem()
				fieldSchema := openapi3.NewObjectSchema()

				var ap openapi3.AdditionalProperties

				if mapValueType.Kind() == reflect.Interface {
					var b = true
					ap.Has = &b
				} else if mapValueType.Kind() == reflect.Struct {
					if !swagger.checkSchemaExist(field.Type.Elem().Name()) {
						swagger.getComponentByModel(reflect.New(field.Type.Elem()).Elem().Interface(), isRequest)
					}
					ap.Schema = openapi3.NewSchemaRef(generateRefName(field.Type.Elem().Name()), nil)
				} else {
					// basic type
					schema := swagger.getBasicSchemaByType(mapValueType.Kind())
					ap.Schema = openapi3.NewSchemaRef("", schema)
				}
				fieldSchema.AdditionalProperties = ap

				schemaRef.Value.Properties[fieldName] = openapi3.NewSchemaRef("", fieldSchema)
			} else if field.Type.Kind() == reflect.Interface {
				schemaRef.Value.Properties[fieldName] = openapi3.NewSchemaRef("", openapi3.NewObjectSchema())
			} else {
				// getSchemaByValue can't distinguish any with map[string]any and []any
				// all of them are nil in func
				fieldSchema := swagger.getSchemaByValue(value.Interface())

				descriptionTag, err := tags.Get(DESCRIPTION)
				if err == nil {
					fieldSchema.Description = descriptionTag.Name
				}

				defaultTag, err := tags.Get(DEFAULT)
				if err == nil {
					fieldSchema.Default = defaultTag.Name
				}

				schemaRef.Value.Properties[fieldName] = openapi3.NewSchemaRef("", fieldSchema)
			}
		}
	}

	if swagger.OpenAPI.Components.Schemas == nil {
		swagger.OpenAPI.Components.Schemas = make(openapi3.Schemas)
	}

	schemaRef.Value.Title = removePackageName(type_.Name())
	// if it goes here, the schemaRef has `Value` rather than `Ref`
	swagger.OpenAPI.Components.Schemas[schemaRef.Value.Title] = schemaRef
}

func (swagger *Swagger) getParametersByModel(model interface{}) openapi3.Parameters {
	parameters := openapi3.NewParameters()
	if model == nil {
		return parameters
	}
	type_ := reflect.TypeOf(model)
	if type_.Kind() == reflect.Ptr {
		type_ = type_.Elem()
	}
	value_ := reflect.ValueOf(model)
	if value_.Kind() == reflect.Ptr {
		value_ = value_.Elem()
	}
	for i := 0; i < type_.NumField(); i++ {
		field := type_.Field(i)
		value := value_.Field(i)
		tags, err := structtag.Parse(string(field.Tag))
		if err != nil {
			panic(err)
		}
		parameter := &openapi3.Parameter{}
		queryTag, err := tags.Get(QUERY)
		if err == nil {
			parameter.In = openapi3.ParameterInQuery
			parameter.Name = queryTag.Name
		}
		uriTag, err := tags.Get(URI)
		if err == nil {
			parameter.In = openapi3.ParameterInPath
			parameter.Name = uriTag.Name
		}
		headerTag, err := tags.Get(HEADER)
		if err == nil {
			parameter.In = openapi3.ParameterInHeader
			parameter.Name = headerTag.Name
		}
		cookieTag, err := tags.Get(COOKIE)
		if err == nil {
			parameter.In = openapi3.ParameterInCookie
			parameter.Name = cookieTag.Name
		}
		if parameter.In == "" {
			continue
		}
		descriptionTag, err := tags.Get(DESCRIPTION)
		if err == nil {
			parameter.Description = descriptionTag.Name
		}
		bindingTag, err := tags.Get(BINDING)
		if err == nil {
			parameter.Required = bindingTag.Name == "required"
		}
		defaultTag, err := tags.Get(DEFAULT)
		schema := swagger.getSchemaByValue(value.Interface())
		if err == nil {
			schema.Default = defaultTag.Name
		}
		parameter.Schema = &openapi3.SchemaRef{
			Value: schema,
		}
		parameters = append(parameters, &openapi3.ParameterRef{
			Value: parameter,
		})
	}
	return parameters
}

func (swagger *Swagger) getSecurity(securities []security.Security) *openapi3.SecurityRequirements {
	securityRequirements := openapi3.NewSecurityRequirements()
	for _, s := range securities {
		swagger.OpenAPI.Components.SecuritySchemes[s.Name()] = &openapi3.SecuritySchemeRef{
			Value: s.Schema(),
		}
		securityRequirements.With(openapi3.NewSecurityRequirement().Authenticate(s.Name()))
	}
	return securityRequirements
}

func (swagger *Swagger) getSchemaByValue(t interface{}) *openapi3.Schema {
	var schema *openapi3.Schema
	var m = float64(0)
	switch t.(type) {
	case int, int8, int16:
		schema = openapi3.NewIntegerSchema()
	case uint, uint8, uint16:
		schema = openapi3.NewIntegerSchema()
		schema.Min = &m
	case int32:
		schema = openapi3.NewInt32Schema()
	case uint32:
		schema = openapi3.NewInt32Schema()
		schema.Min = &m
	case int64:
		schema = openapi3.NewInt64Schema()
	case uint64:
		schema = openapi3.NewInt64Schema()
		schema.Min = &m
	case string:
		schema = openapi3.NewStringSchema()
	case time.Time:
		schema = openapi3.NewDateTimeSchema()
	case float32:
		schema = openapi3.NewFloat64Schema()
		schema.Format = "float"
	case float64:
		schema = openapi3.NewFloat64Schema()
		schema.Format = "double"
	case bool:
		schema = openapi3.NewBoolSchema()
	case []byte:
		schema = openapi3.NewBytesSchema()
	case *multipart.FileHeader:
		schema = openapi3.NewStringSchema()
		schema.Format = "binary"
	case []*multipart.FileHeader:
		schema = openapi3.NewArraySchema()
		schema.Items = &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Type:   "string",
				Format: "binary",
			},
		}
	}
	return schema
}

func parseEnumTag(enumTag string) map[string]string {
	parts := strings.Split(enumTag, ";")
	var res = make(map[string]string)
	for _, part := range parts {
		split := strings.Split(part, ":")
		k := strings.TrimSpace(split[0])
		v := strings.TrimSpace(split[1])
		res[k] = v
	}
	return res
}

func (swagger *Swagger) getEnumComponentByTag(name, enumTag string) {
	parts := parseEnumTag(enumTag)

	typ, ok := parts["type"]
	if !ok {
		// default is string
		typ = "string"
	}

	var values []any
	vs, ok := parts["values"]
	if !ok {
		panic("enum tag must have values")
	}
	split := strings.Split(vs, ",")
	for _, v := range split {
		values = append(values, strings.TrimSpace(v))
	}

	var enumSchema = openapi3.Schema{
		Type:        typ,
		Title:       name,
		Description: parts["description"],
		Enum:        values,
	}

	if swagger.OpenAPI.Components.Schemas == nil {
		swagger.OpenAPI.Components.Schemas = make(openapi3.Schemas)
	}

	swagger.OpenAPI.Components.Schemas[name] = openapi3.NewSchemaRef("", &enumSchema)
}

func (swagger *Swagger) getEnumComponent(enum router.Enum) {
	for enumName, enumItem := range enum {
		var enumSchema = openapi3.Schema{
			Type:        enumItem.Kind,
			Title:       enumName,
			Description: enumItem.Description,
			Enum:        enumItem.Values,
		}

		swagger.OpenAPI.Components.Schemas[enumName] = openapi3.NewSchemaRef("", &enumSchema)
	}
}

func (swagger *Swagger) getBasicSchemaByType(typ reflect.Kind) *openapi3.Schema {
	var schema *openapi3.Schema
	var m = float64(0)
	switch typ {
	case reflect.Int, reflect.Int8, reflect.Int16:
		schema = openapi3.NewIntegerSchema()
	case reflect.Uint, reflect.Uint8, reflect.Uint16:
		schema = openapi3.NewIntegerSchema()
		schema.Min = &m
	case reflect.Int32:
		schema = openapi3.NewInt32Schema()
	case reflect.Uint32:
		schema = openapi3.NewInt32Schema()
		schema.Min = &m
	case reflect.Int64:
		schema = openapi3.NewInt64Schema()
	case reflect.Uint64:
		schema = openapi3.NewInt64Schema()
		schema.Min = &m
	case reflect.String:
		schema = openapi3.NewStringSchema()
	case reflect.Float32:
		schema = openapi3.NewFloat64Schema()
		schema.Format = "float"
	case reflect.Float64:
		schema = openapi3.NewFloat64Schema()
		schema.Format = "double"
	case reflect.Bool:
		schema = openapi3.NewBoolSchema()
	}
	return schema
}

func (swagger *Swagger) checkSchemaExist(name string) bool {
	for _, schema := range swagger.OpenAPI.Components.Schemas {
		if schema.Value != nil && schema.Value.Title == name {
			return true
		}
	}

	return false
}

func generateRefName(structName string) string {
	return "#/components/schemas/" + structName
}

func removePackageName(name string) string {
	split := strings.Split(name, ".")
	return split[len(split)-1]
}

func (swagger *Swagger) fixPath(path string) string {
	reg := regexp.MustCompile("/:([0-9a-zA-Z]+)")
	return reg.ReplaceAllString(path, "/{${1}}")
}

func isBuiltinType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String,
		reflect.UnsafePointer:
		return true
	default:
		return false
	}
}
