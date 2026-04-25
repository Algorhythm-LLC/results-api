package openapispec

import _ "embed"

// YAML is the embedded OpenAPI 3.1 document (source: openapi.yaml in this package).
//
//go:embed openapi.yaml
var YAML []byte
