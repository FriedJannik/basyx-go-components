package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOpenAPITemplates(t *testing.T) {
	spec := []byte(`openapi: 3.0.3
paths:
  /submodels:
    post:
      operationId: PostSubmodel
      summary: Creates a new Submodel
      requestBody:
        required: true
    get:
      operationId: GetAllSubmodels
      parameters:
        - name: limit
          in: query
          required: false
  /submodels/{id}:
    get:
      operationId: GetSubmodelById
`)
	templates, err := ParseOpenAPITemplates(spec)
	require.NoError(t, err)
	require.Len(t, templates, 3)
	require.Equal(t, "GET /submodels", templates[0].ID)
	require.Equal(t, "limit", templates[0].Parameters[0].Name)
	require.Equal(t, "POST /submodels", templates[1].ID)
	require.True(t, templates[1].HasBody)
	require.Equal(t, "GET /submodels/{id}", templates[2].ID)
}

func TestLoadTemplatesFromFileResolvesRepoRelativePathFromNestedWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "cmd", "benchmarkservice")
	specPath := filepath.Join(root, "cmd", "submodelrepositoryservice", "openapi.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o750))
	require.NoError(t, os.MkdirAll(nested, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o600))
	require.NoError(t, os.WriteFile(specPath, []byte(`openapi: 3.0.3
paths:
  /submodels:
    get:
      operationId: GetAllSubmodels
`), 0o600))

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nested))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousWD))
	})

	templates, err := LoadTemplatesFromFile("cmd/submodelrepositoryservice/openapi.yaml")
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, "GET /submodels", templates[0].ID)
}
