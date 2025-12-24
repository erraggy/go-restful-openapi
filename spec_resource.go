package restfulspec

import (
	"log"

	restful "github.com/emicklei/go-restful/v3"
	"github.com/erraggy/oastools/parser"
	"github.com/go-openapi/spec"
)

// NewOpenAPIService returns a new WebService that provides the API documentation of all services
// conforming to the OpenAPI documentation specification.
//
// The builder used depends on config.OASVersion:
//   - OASVersion >= OASVersion300: Uses BuildOAS3 (oastools library, OAS 3.x output)
//   - OASVersion == OASVersion20: Uses BuildOAS2 (oastools library, OAS 2.0 output)
//   - OASVersion unset (zero): Uses BuildSwagger (legacy go-openapi/spec, OAS 2.0 output)
func NewOpenAPIService(config Config) *restful.WebService {
	ws := new(restful.WebService)
	ws.Path(config.APIPath)
	ws.Produces(restful.MIME_JSON)
	if !config.DisableCORS {
		ws.Filter(enableCORS)
	}

	// Build the appropriate document based on OASVersion
	var doc any
	switch {
	case config.OASVersion >= parser.OASVersion300:
		d, err := BuildOAS3(config)
		if err != nil {
			log.Printf("restfulspec: failed to build OAS 3.x document: %v", err)
			return ws
		}
		doc = d
	case config.OASVersion == parser.OASVersion20:
		d, err := BuildOAS2(config)
		if err != nil {
			log.Printf("restfulspec: failed to build OAS 2.0 document: %v", err)
			return ws
		}
		doc = d
	default:
		doc = BuildSwagger(config)
	}

	ws.Route(ws.GET("/").To(func(req *restful.Request, resp *restful.Response) {
		_ = resp.WriteAsJson(doc)
	}))
	return ws
}

// BuildSwagger returns a Swagger object for all services' API endpoints.
func BuildSwagger(config Config) *spec.Swagger {
	// collect paths and model definitions to build Swagger object.
	paths := &spec.Paths{Paths: map[string]spec.PathItem{}}
	definitions := spec.Definitions{}

	for _, each := range config.WebServices {
		for path, item := range buildPaths(each, config).Paths {
			existingPathItem, ok := paths.Paths[path]
			if ok {
				for _, r := range each.Routes() {
					_, patterns := sanitizePath(r.Path)
					item = buildPathItem(each, r, existingPathItem, patterns, config)
				}
			}
			paths.Paths[path] = item
		}
		for name, def := range buildDefinitions(each, config) {
			definitions[name] = def
		}
	}
	swagger := &spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Host:        config.Host,
			Schemes:     config.Schemes,
			Swagger:     "2.0",
			Paths:       paths,
			Definitions: definitions,
		},
	}
	if config.PostBuildSwaggerObjectHandler != nil {
		config.PostBuildSwaggerObjectHandler(swagger)
	}
	return swagger
}

func enableCORS(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if origin := req.HeaderParameter(restful.HEADER_Origin); origin != "" {
		// prevent duplicate header
		if len(resp.Header().Get(restful.HEADER_AccessControlAllowOrigin)) == 0 {
			resp.AddHeader(restful.HEADER_AccessControlAllowOrigin, origin)
		}
	}
	chain.ProcessFilter(req, resp)
}
