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
