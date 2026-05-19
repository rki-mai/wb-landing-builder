package config

import (
	"github.com/danielgtaylor/huma/v2"
)

func GetHumaConfig(url string) huma.Config {
	return huma.Config{
		OpenAPIPath:  "/openapi.json",
		DocsPath:     "/docs",
		DocsRenderer: huma.DocsRendererStoplightElements,
		Formats: map[string]huma.Format{
			"application/json": huma.DefaultJSONFormat,
			"json":             huma.DefaultJSONFormat,
		},
		OpenAPI: &huma.OpenAPI{
			Info: &huma.Info{
				Title:       "WB Landing Builder API",
				Version:     "1.0.0",
				Description: "API for building and managing landing pages with authentication and draft management",
				Contact: &huma.Contact{
					Name:  "RKI-MAI Team",
					Email: "rki-mai@example.com",
					URL:   "https://github.com/rki-mai/wb-landing-builder",
				},
				License: &huma.License{
					Name: "MIT",
					URL:  "https://opensource.org/licenses/MIT",
				},
				TermsOfService: "https://example.com/terms",
			},
			Servers: []*huma.Server{
				{
					URL:         url,
					Description: "Local development server",
				},
			},
			Components: &huma.Components{
				SecuritySchemes: map[string]*huma.SecurityScheme{
					"BearerAuth": {
						Type:         "http",
						Scheme:       "bearer",
						BearerFormat: "JWT",
						Description:  "Some api endpoints require JWT token. Obtain it via /auth/login.",
					},
				},
			},
			ExternalDocs: &huma.ExternalDocs{
				Description: "Full project documentation and architecture overview",
				URL:         "https://docs.wb-landing-builder.example.com",
			},
			Security: []map[string][]string{
				{"BearerAuth": {}},
			},
		},
	}
}
