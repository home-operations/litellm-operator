package litellmclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPCredentialErrorsAreRedacted(t *testing.T) {
	token := "installation-token/+\"&"
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				assert.Equal(t, method, req.Method)
				w.WriteHeader(http.StatusBadRequest)
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{
					"raw": token, "escaped": url.QueryEscape(token),
				}))
			}))
			defer srv.Close()
			c := New(srv.URL, "sk-master", srv.Client())
			body := map[string]any{"credentials": map[string]any{"auth_value": token}}
			var err error
			if method == http.MethodPost {
				err = c.CreateMCPServer(t.Context(), body)
			} else {
				err = c.UpdateMCPServer(t.Context(), body)
			}
			require.Error(t, err)
			encoded, marshalErr := json.Marshal(token)
			require.NoError(t, marshalErr)
			assert.NotContains(t, err.Error(), token)
			assert.NotContains(t, err.Error(), string(encoded[1:len(encoded)-1]))
			assert.NotContains(t, err.Error(), url.QueryEscape(token))
			assert.Contains(t, err.Error(), "[REDACTED]")
		})
	}
}
