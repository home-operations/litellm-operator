package controller

import (
	"context"
	"encoding/json"

	"github.com/home-operations/litellm-operator/internal/litellmclient"
)

const (
	managedByKey   = "managed_by"
	managedByValue = "litellm-operator"
)

// modelAPI is the slice of the LiteLLM admin client the model sync needs.
type modelAPI interface {
	ListModels(ctx context.Context) ([]litellmclient.Model, error)
	CreateModel(ctx context.Context, m litellmclient.Model) error
	UpdateModel(ctx context.Context, m litellmclient.Model) error
	DeleteModel(ctx context.Context, id string) error
}

// syncModels reconciles the proxy's DB-backed models to match the desired
// rendered entries. It only touches models tagged as operator-managed, so
// models added out-of-band (UI, other tools) are left alone.
func syncModels(ctx context.Context, c modelAPI, desired []map[string]any) error {
	existing, err := c.ListModels(ctx)
	if err != nil {
		return err
	}

	// Each managed name is reconciled against its first listed row; any further
	// managed rows under that name are surplus and deleted, so a duplicate left
	// by an interrupted sync doesn't keep serving alongside the reconciled row.
	// Rows are keyed by id first so a row listed twice is never queued for
	// deletion as a duplicate of itself.
	managed := make(map[string]litellmclient.Model, len(existing))
	seen := make(map[string]struct{}, len(existing))
	var surplus []string
	for _, m := range existing {
		if m.ModelInfo[managedByKey] != managedByValue {
			continue
		}
		id := m.ModelID()
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, taken := managed[m.ModelName]; taken {
			surplus = append(surplus, id)
			continue
		}
		managed[m.ModelName] = m
	}

	desiredNames := make(map[string]struct{}, len(desired))
	for _, entry := range desired {
		name, _ := entry[keyModelName].(string)
		desiredNames[name] = struct{}{}

		params, _ := entry[keyLitellmParams].(map[string]any)
		info, _ := entry["model_info"].(map[string]any)
		if info == nil {
			info = map[string]any{}
		}
		info[managedByKey] = managedByValue
		model := litellmclient.Model{ModelName: name, LiteLLMParams: params, ModelInfo: info}

		cur, ok := managed[name]
		if !ok {
			if err := c.CreateModel(ctx, model); err != nil {
				return err
			}
			continue
		}
		if subsetEqual(params, cur.LiteLLMParams) && subsetEqual(info, cur.ModelInfo) {
			continue
		}
		info["id"] = cur.ModelID()
		if err := c.UpdateModel(ctx, model); err != nil {
			return err
		}
	}

	for name, cur := range managed {
		if _, keep := desiredNames[name]; keep {
			continue
		}
		surplus = append(surplus, cur.ModelID())
	}
	for _, id := range surplus {
		if err := c.DeleteModel(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// subsetEqual reports whether every key in desired is present and equal in
// existing. The proxy echoes back extra litellm-added fields (and merges
// model_info with its cost map), so a subset comparison avoids treating those
// as drift.
func subsetEqual(desired, existing map[string]any) bool {
	for k, v := range desired {
		ev, ok := existing[k]
		if !ok || !jsonEqual(v, ev) {
			return false
		}
	}
	return true
}

func jsonEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ab) == string(bb)
}
