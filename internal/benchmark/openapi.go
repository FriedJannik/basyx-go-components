// Package benchmark implements the browser-controlled BaSyx REST benchmark service.
package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

var supportedMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true,
}

// LoadTemplatesFromFile parses request templates from a developer-selected OpenAPI file.
func LoadTemplatesFromFile(path string) ([]RequestTemplate, error) {
	resolvedPath, err := resolveDeveloperPath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(resolvedPath) //nolint:gosec // benchmark developers intentionally provide local OpenAPI spec paths.
	if err != nil {
		return nil, fmt.Errorf("BENCH-TEMPLATE-READSPEC: %w", err)
	}
	return ParseOpenAPITemplates(data)
}

func resolveDeveloperPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("BENCH-TEMPLATE-EMPTYSPEC: OpenAPI spec path is required")
	}
	if filepath.IsAbs(path) || fileExists(path) {
		return path, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("BENCH-TEMPLATE-GETWD: %w", err)
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, path)
		if fileExists(candidate) {
			return candidate, nil
		}
		if fileExists(filepath.Join(dir, "go.mod")) {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return path, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path) //nolint:gosec // developer-selected spec paths are resolved for local benchmark use.
	return err == nil && !info.IsDir()
}

// ParseOpenAPITemplates generates configurable request templates from OpenAPI bytes.
func ParseOpenAPITemplates(data []byte) ([]RequestTemplate, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("BENCH-TEMPLATE-PARSEYAML: %w", err)
	}
	pathsNode, ok := doc["paths"].(map[string]any)
	if !ok || len(pathsNode) == 0 {
		return nil, fmt.Errorf("BENCH-TEMPLATE-NOPATHS: OpenAPI spec does not contain paths")
	}

	paths := make([]string, 0, len(pathsNode))
	for path := range pathsNode {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var templates []RequestTemplate
	for _, path := range paths {
		pathItem, ok := pathsNode[path].(map[string]any)
		if !ok {
			continue
		}
		templates = append(templates, templatesFromPathItem(path, pathItem)...)
	}
	return templates, nil
}

func templatesFromPathItem(path string, pathItem map[string]any) []RequestTemplate {
	methods := make([]string, 0, len(pathItem))
	for method := range pathItem {
		method = strings.ToLower(method)
		if supportedMethods[method] {
			methods = append(methods, method)
		}
	}
	sort.Strings(methods)

	templates := make([]RequestTemplate, 0, len(methods))
	for _, method := range methods {
		op, ok := pathItem[method].(map[string]any)
		if !ok {
			continue
		}
		templates = append(templates, RequestTemplate{
			ID:          strings.ToUpper(method) + " " + path,
			OperationID: stringValue(op["operationId"]),
			Summary:     stringValue(op["summary"]),
			Method:      strings.ToUpper(method),
			Path:        path,
			HasBody:     op["requestBody"] != nil,
			Parameters:  parseTemplateParams(op["parameters"]),
			Headers:     map[string]string{"Content-Type": "application/json"},
		})
	}
	return templates
}

func parseTemplateParams(raw any) []TemplateParam {
	rawParams, ok := raw.([]any)
	if !ok {
		return nil
	}
	params := make([]TemplateParam, 0, len(rawParams))
	for _, rawParam := range rawParams {
		param, ok := rawParam.(map[string]any)
		if !ok {
			continue
		}
		params = append(params, TemplateParam{
			Name:     stringValue(param["name"]),
			In:       stringValue(param["in"]),
			Required: boolValue(param["required"]),
		})
	}
	return params
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func boolValue(value any) bool {
	boolean, _ := value.(bool)
	return boolean
}
