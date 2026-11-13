package giteasearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Setenv("UPDATECLI_GITEA_USERNAME", "")
	t.Setenv("UPDATECLI_GITEA_TOKEN", "test-token")
	for _, tt := range []struct {
		name      string
		spec      map[string]any
		wantLimit int
		wantErr   string
	}{
		{name: "Defaults", spec: map[string]any{"url": "gitea.example.com", "owner": "updatecli"}, wantLimit: 10},
		{name: "Missing owner", spec: map[string]any{"url": "gitea.example.com"}, wantErr: ErrOwnerEmpty},
		{name: "Blank owner", spec: map[string]any{"url": "gitea.example.com", "owner": " "}, wantErr: ErrOwnerEmpty},
		{name: "Missing URL", spec: map[string]any{"owner": "updatecli"}, wantErr: "wrong configuration"},
		{name: "Negative limit remains unlimited", spec: map[string]any{"url": "gitea.example.com", "owner": "updatecli", "limit": -1}, wantLimit: -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g, err := New(tt.spec)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "https://gitea.example.com", g.spec.URL)
			assert.Equal(t, "^main$", g.spec.Branch)
			assert.Equal(t, tt.wantLimit, g.limit)
			assert.Equal(t, "oauth2", g.spec.Username)
			assert.Equal(t, "test-token", g.spec.Token)
		})
	}
}
