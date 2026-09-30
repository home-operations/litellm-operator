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

const (
	testAppMetadataValue  = "example"
	testModelsJSONField   = "models"
	testKeyJSONField      = "key"
	testKeyAlias          = "application"
	testAppMetadataField  = "app"
	testLiveKey           = "sk-live"
	testKeyAliasJSONField = "key_alias"
	testUpdatedModel      = "new"
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

func TestClient_GenerateAndDeleteVirtualKey(t *testing.T) {
	type call struct {
		path string
		body map[string]any
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		calls = append(calls, call{path: r.URL.Path, body: body})
		if r.URL.Path == "/key/generate" {
			_ = json.NewEncoder(w).Encode(map[string]string{testKeyJSONField: "sk-generated"})
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "master", srv.Client())
	generated, err := c.GenerateVirtualKey(context.Background(), VirtualKeyRequest{
		KeyAlias: testKeyAlias,
		Models:   []string{"openai/gpt-5"},
		Metadata: map[string]string{testAppMetadataField: testAppMetadataValue},
	})
	require.NoError(t, err)
	assert.Equal(t, "sk-generated", generated.Key)
	require.NoError(t, c.DeleteVirtualKey(context.Background(), generated.Key))

	require.Len(t, calls, 2)
	assert.Equal(t, "/key/generate", calls[0].path)
	assert.Equal(t, testKeyAlias, calls[0].body[testKeyAliasJSONField])
	assert.Equal(t, []any{"openai/gpt-5"}, calls[0].body[testModelsJSONField])
	assert.Equal(t, "/key/delete", calls[1].path)
	assert.Equal(t, []any{"sk-generated"}, calls[1].body["keys"])
}

func TestClient_GetAndUpdateVirtualKey(t *testing.T) {
	var update map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key/info" {
			assert.Equal(t, testLiveKey, r.URL.Query().Get(testKeyJSONField))
			assert.Equal(t, http.MethodGet, r.Method)
			_ = json.NewEncoder(w).Encode(map[string]any{
				testKeyJSONField: testLiveKey,
				"info":           map[string]any{testKeyAliasJSONField: testKeyAlias, testModelsJSONField: []string{"old"}},
			})
			return
		}
		require.Equal(t, "/key/update", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&update))
	}))
	defer srv.Close()

	c := New(srv.URL, "master", srv.Client())
	live, err := c.GetVirtualKey(context.Background(), testLiveKey)
	require.NoError(t, err)
	assert.Equal(t, testKeyAlias, live.KeyAlias)
	assert.Equal(t, []string{"old"}, live.Models)
	require.NoError(t, c.UpdateVirtualKey(context.Background(), testLiveKey, VirtualKeyRequest{Models: []string{testUpdatedModel}}, false))
	assert.Equal(t, testLiveKey, update[testKeyJSONField])
	assert.Equal(t, []any{testUpdatedModel}, update[testModelsJSONField])
}

func TestClient_GetVirtualKeyRejectsMissingInfo(t *testing.T) {
	for _, body := range []string{`{}`, `{"info":null}`, `{"key_alias":"application"}`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			_, err := New(srv.URL, "master", srv.Client()).GetVirtualKey(t.Context(), testLiveKey)
			require.ErrorContains(t, err, "response has no info")
		})
	}
}

func TestClient_UpdateVirtualKeyClearsSettings(t *testing.T) {
	tests := []struct {
		name    string
		request VirtualKeyRequest
	}{
		{name: "nil collections"},
		{name: "empty collections", request: VirtualKeyRequest{
			Models: []string{}, Aliases: map[string]string{}, Metadata: map[string]string{},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			}))
			defer srv.Close()
			require.NoError(t, New(srv.URL, "master", srv.Client()).UpdateVirtualKey(t.Context(), testLiveKey, tt.request, true))
			assert.Equal(t, map[string]any{
				testKeyJSONField: testLiveKey, testModelsJSONField: []any{}, "aliases": map[string]any{}, "metadata": map[string]any{},
				testKeyAliasJSONField: nil, "user_id": nil, teamIDJSONKey: nil, "duration": nil, "budget_duration": nil,
				"max_budget": nil, "max_parallel_requests": nil, "tpm_limit": nil, "rpm_limit": nil,
			}, body)
		})
	}
}

func TestClient_UpdateVirtualKeySendsValuesAndPreservesExpiry(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
	}))
	defer srv.Close()
	request := VirtualKeyRequest{
		KeyAlias: testKeyAlias, Models: []string{testUpdatedModel}, Aliases: map[string]string{"alias": testUpdatedModel},
		UserID: "user", TeamID: "team", Duration: "30d", BudgetDuration: "1d",
		MaxBudget: new(12.5), MaxParallelRequests: new(int64(3)), TPMLimit: new(int64(100)), RPMLimit: new(int64(0)),
		Metadata: map[string]string{testAppMetadataField: testAppMetadataValue},
	}
	require.NoError(t, New(srv.URL, "master", srv.Client()).UpdateVirtualKey(t.Context(), testLiveKey, request, false))
	assert.Equal(t, map[string]any{
		testKeyJSONField: testLiveKey, testKeyAliasJSONField: testKeyAlias, testModelsJSONField: []any{testUpdatedModel},
		"aliases": map[string]any{"alias": testUpdatedModel}, "user_id": "user", teamIDJSONKey: "team", "budget_duration": "1d",
		"max_budget": 12.5, "max_parallel_requests": float64(3), "tpm_limit": float64(100), "rpm_limit": float64(0),
		"metadata": map[string]any{testAppMetadataField: testAppMetadataValue},
	}, body)
}

func TestClient_ManageTeam(t *testing.T) {
	const (
		teamID         = "platform"
		userID         = "user"
		teamMemberRole = "user"
	)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/team/new" {
			_ = json.NewEncoder(w).Encode(map[string]string{teamIDJSONKey: teamID})
		}
		if r.URL.Path == "/team/info" {
			_ = json.NewEncoder(w).Encode(map[string]any{"team_info": map[string]any{teamIDJSONKey: teamID, "members_with_roles": []any{}}})
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "master", srv.Client())
	team, err := c.CreateTeam(context.Background(), TeamRequest{TeamID: teamID, Models: []string{}})
	require.NoError(t, err)
	assert.Equal(t, teamID, team.TeamID)
	require.NoError(t, c.UpdateTeam(context.Background(), TeamRequest{TeamID: team.TeamID, Models: []string{}}))
	_, err = c.GetTeam(context.Background(), team.TeamID)
	require.NoError(t, err)
	require.NoError(t, c.AddTeamMember(context.Background(), team.TeamID, TeamMember{UserID: userID, Role: teamMemberRole}))
	require.NoError(t, c.DeleteTeamMember(context.Background(), team.TeamID, TeamMember{UserID: userID}))
	require.NoError(t, c.DeleteTeam(context.Background(), team.TeamID))
	assert.Equal(t, []string{"/team/new", "/team/update", "/team/info", "/team/member_add", "/team/member_delete", "/team/delete"}, paths)
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
