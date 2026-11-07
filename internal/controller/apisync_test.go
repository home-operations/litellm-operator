package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/home-operations/litellm-operator/internal/litellmclient"
)

type fakeModelAPI struct {
	existing []litellmclient.Model
	created  []litellmclient.Model
	updated  []litellmclient.Model
	deleted  []string
}

func (f *fakeModelAPI) ListModels(context.Context) ([]litellmclient.Model, error) {
	return f.existing, nil
}
func (f *fakeModelAPI) CreateModel(_ context.Context, m litellmclient.Model) error {
	f.created = append(f.created, m)
	return nil
}
func (f *fakeModelAPI) UpdateModel(_ context.Context, m litellmclient.Model) error {
	f.updated = append(f.updated, m)
	return nil
}
func (f *fakeModelAPI) DeleteModel(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func managedModel(name, id, model string) litellmclient.Model {
	return litellmclient.Model{
		ModelName:     name,
		ModelInfo:     map[string]any{"id": id, managedByKey: managedByValue},
		LiteLLMParams: map[string]any{keyModel: model},
	}
}

func desiredModel(name, model string) map[string]any {
	return map[string]any{keyModelName: name, keyLitellmParams: map[string]any{keyModel: model}}
}

func TestSyncModels_CreatesDeletesAndLeavesUnmanagedAlone(t *testing.T) {
	f := &fakeModelAPI{existing: []litellmclient.Model{
		managedModel("stale", "1", "openai/stale"),
		managedModel("keep", "2", "openai/keep"),
		{ModelName: "manual", ModelInfo: map[string]any{"id": "3"}, LiteLLMParams: map[string]any{keyModel: "openai/manual"}},
	}}

	desired := []map[string]any{desiredModel("new", "openai/new"), desiredModel("keep", "openai/keep")}
	require.NoError(t, syncModels(context.Background(), f, desired))

	require.Len(t, f.created, 1)
	assert.Equal(t, "new", f.created[0].ModelName)
	assert.Equal(t, managedByValue, f.created[0].ModelInfo[managedByKey])

	// "keep" is unchanged -> no update; "manual" is not operator-managed -> never touched.
	assert.Empty(t, f.updated)
	assert.Equal(t, []string{"1"}, f.deleted)
}

func TestSyncModels_UpdatesChangedModelWithExistingID(t *testing.T) {
	f := &fakeModelAPI{existing: []litellmclient.Model{managedModel("keep", "42", "openai/old")}}
	desired := []map[string]any{desiredModel("keep", "openai/new")}
	require.NoError(t, syncModels(context.Background(), f, desired))

	assert.Empty(t, f.created)
	assert.Empty(t, f.deleted)
	require.Len(t, f.updated, 1)
	assert.Equal(t, "42", f.updated[0].ModelInfo["id"], "update must target the existing model id")
	assert.Equal(t, "openai/new", f.updated[0].LiteLLMParams[keyModel])
}

func TestSyncModels_UpdatesChangedModelInfo(t *testing.T) {
	existing := managedModel("keep", "42", "openai/same")
	existing.ModelInfo["max_input_tokens"] = 32768
	existing.ModelInfo["litellm_provider"] = "openai"
	unchanged := managedModel("other", "43", "openai/other")
	unchanged.ModelInfo["max_input_tokens"] = 65536
	f := &fakeModelAPI{existing: []litellmclient.Model{existing, unchanged}}

	changed := desiredModel("keep", "openai/same")
	changed["model_info"] = map[string]any{"max_input_tokens": 196608}
	same := desiredModel("other", "openai/other")
	same["model_info"] = map[string]any{"max_input_tokens": 65536}
	require.NoError(t, syncModels(context.Background(), f, []map[string]any{changed, same}))

	assert.Empty(t, f.created)
	assert.Empty(t, f.deleted)
	require.Len(t, f.updated, 1, "only the model whose model_info changed is updated")
	assert.Equal(t, "42", f.updated[0].ModelInfo["id"])
	assert.Equal(t, 196608, f.updated[0].ModelInfo["max_input_tokens"])
}

func TestSyncModels_SameNameRows(t *testing.T) {
	tests := []struct {
		name        string
		existing    []litellmclient.Model
		desired     []map[string]any
		wantUpdated []string
		wantDeleted []string
	}{
		{
			name:     "wildcard model listed once is left alone",
			existing: []litellmclient.Model{managedModel("zai/*", "1", "zai/*")},
			desired:  []map[string]any{desiredModel("zai/*", "zai/*")},
		},
		{
			name: "surplus managed rows for a desired name are deleted",
			existing: []litellmclient.Model{
				managedModel("zai/*", "1", "zai/*"),
				managedModel("zai/*", "2", "zai/*"),
				managedModel("zai/*", "3", "zai/*"),
			},
			desired:     []map[string]any{desiredModel("zai/*", "zai/*")},
			wantDeleted: []string{"2", "3"},
		},
		{
			name: "the first row is the one reconciled",
			existing: []litellmclient.Model{
				managedModel("keep", "1", "openai/old"),
				managedModel("keep", "2", "openai/new"),
			},
			desired:     []map[string]any{desiredModel("keep", "openai/new")},
			wantUpdated: []string{"1"},
			wantDeleted: []string{"2"},
		},
		{
			name: "every row of an undesired name is deleted",
			existing: []litellmclient.Model{
				managedModel("stale", "1", "openai/stale"),
				managedModel("stale", "2", "openai/stale"),
			},
			wantDeleted: []string{"2", "1"},
		},
		{
			name: "a row listed twice is not deleted as its own duplicate",
			existing: []litellmclient.Model{
				managedModel("keep", "1", "openai/keep"),
				managedModel("keep", "1", "openai/keep"),
			},
			desired: []map[string]any{desiredModel("keep", "openai/keep")},
		},
		{
			name: "an unmanaged row sharing a desired name is left alone",
			existing: []litellmclient.Model{
				{ModelName: "keep", ModelInfo: map[string]any{"id": "9"}, LiteLLMParams: map[string]any{keyModel: "openai/manual"}},
				managedModel("keep", "1", "openai/keep"),
			},
			desired: []map[string]any{desiredModel("keep", "openai/keep")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeModelAPI{existing: tt.existing}
			require.NoError(t, syncModels(context.Background(), f, tt.desired))

			assert.Empty(t, f.created)
			var updated []string
			for _, m := range f.updated {
				updated = append(updated, m.ModelID())
			}
			assert.Equal(t, tt.wantUpdated, updated)
			assert.Equal(t, tt.wantDeleted, f.deleted)
		})
	}
}
