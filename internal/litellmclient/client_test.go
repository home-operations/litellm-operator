package litellmclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListModelsParsesDataAndAuth(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		assert.Equal(t, "/model/info", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{
				"model_name":     "glm",
				"model_info":     map[string]any{"id": "abc", "managed_by": "litellm-operator"},
				"litellm_params": map[string]any{"model": "openai/glm"},
			}},
		})
	}))
	defer srv.Close()

	models, err := New(srv.URL, "sk-master", srv.Client()).ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "glm", models[0].ModelName)
	assert.Equal(t, "abc", models[0].ModelID())
	assert.Equal(t, "Bearer sk-master", auth)
}

func TestClient_CreateUpdateDeleteSendCorrectRequests(t *testing.T) {
	type call struct {
		method, path string
		body         map[string]any
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls = append(calls, call{r.Method, r.URL.Path, body})
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := New(srv.URL, "k", srv.Client())
	ctx := context.Background()

	require.NoError(t, c.CreateModel(ctx, Model{ModelName: "m", LiteLLMParams: map[string]any{"model": "openai/m"}}))
	require.NoError(t, c.UpdateModel(ctx, Model{ModelName: "m", ModelInfo: map[string]any{"id": "x"}}))
	require.NoError(t, c.DeleteModel(ctx, "x"))

	require.Len(t, calls, 3)
	assert.Equal(t, "/model/new", calls[0].path)
	assert.Equal(t, "openai/m", calls[0].body["litellm_params"].(map[string]any)["model"])
	assert.Equal(t, "/model/update", calls[1].path)
	assert.Equal(t, "/model/delete", calls[2].path)
	assert.Equal(t, "x", calls[2].body["id"])
}

func TestClient_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	err := New(srv.URL, "k", srv.Client()).DeleteModel(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "boom")
}
