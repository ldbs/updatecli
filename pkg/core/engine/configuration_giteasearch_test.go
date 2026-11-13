package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	giteasdk "gitea.dev/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/updatecli/updatecli/pkg/core/engine/manifest"
	"github.com/updatecli/updatecli/pkg/plugins/resources/gitea/pullrequest"
	"github.com/updatecli/updatecli/pkg/plugins/scms/gitea"
)

func TestLoadConfigurationsGiteaSearch(t *testing.T) {
	t.Setenv("UPDATECLI_GITEA_USERNAME", "")
	for _, tt := range []struct {
		name        string
		branch      string
		secondOwner string
		empty       bool
		wantPairs   []string
	}{
		{
			name:      "One search",
			branch:    "^(main|stable)$",
			wantPairs: []string{"one/main", "one/stable", "two/main", "two/stable"},
		},
		{
			name:        "Two searches",
			branch:      "^main$",
			secondOwner: "other",
			wantPairs: []string{
				"one/main+three/main", "one/main+four/main", "one/main+five/main",
				"two/main+three/main", "two/main+four/main", "two/main+five/main",
			},
		},
		{
			name:   "No matching repository",
			branch: "^main$",
			empty:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lists := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
				switch r.URL.Path {
				case "/api/v1/users/updatecli/repos":
					lists++
					if tt.empty {
						_, _ = w.Write([]byte(`[]`))
					} else {
						_ = json.NewEncoder(w).Encode([]giteasdk.Repository{{Name: "one"}, {Name: "two"}})
					}
				case "/api/v1/repos/updatecli/one/branches", "/api/v1/repos/updatecli/two/branches":
					_ = json.NewEncoder(w).Encode([]giteasdk.Branch{{Name: "main"}, {Name: "stable"}})
				case "/api/v1/users/other/repos":
					_ = json.NewEncoder(w).Encode([]giteasdk.Repository{{Name: "three"}, {Name: "four"}, {Name: "five"}})
				case "/api/v1/repos/other/three/branches", "/api/v1/repos/other/four/branches", "/api/v1/repos/other/five/branches":
					_ = json.NewEncoder(w).Encode([]giteasdk.Branch{{Name: "main"}})
				default:
					t.Errorf("unexpected request: %s", r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			secondSCM := ""
			if tt.secondOwner != "" {
				secondSCM = fmt.Sprintf(`  second:
    kind: giteasearch
    spec:
      url: %s
      token: test-token
      owner: %s
`, server.URL, tt.secondOwner)
			}
			filename := filepath.Join(t.TempDir(), "updatecli.yaml")
			require.NoError(t, os.WriteFile(filename, []byte(fmt.Sprintf(`name: Central policy
options:
  relativepaths: manifest
scms:
  default:
    kind: giteasearch
    spec:
      url: %s
      token: test-token
      owner: updatecli
      branch: %s
%s
sources:
  version:
    kind: shell
    spec:
      command: echo 2
targets:
  version:
    kind: yaml
    scmid: default
    sourceid: version
    spec:
      file: version.yaml
      key: $.version
actions:
  default:
    kind: gitea/pullrequest
    scmid: default
    title: Central update
    spec:
      assignees: [alice]
`, server.URL, tt.branch, secondSCM)), 0600))
			e := Engine{Options: Options{Manifests: []manifest.Manifest{{Manifests: []string{filename}}}}}
			err := e.LoadConfigurations()
			if tt.secondOwner == "" {
				assert.Equal(t, 1, lists)
			}
			if tt.empty {
				require.ErrorContains(t, err, `no scm discovered for giteasearch scm "default"`)
				return
			}
			require.NoError(t, err)
			require.Len(t, e.Pipelines, len(tt.wantPairs))
			var found []string
			for _, p := range e.Pipelines {
				s := p.SCMs["default"].Handler.(*gitea.Gitea)
				pair := s.Spec.Repository + "/" + s.Spec.Branch
				if tt.secondOwner != "" {
					second := p.SCMs["second"].Handler.(*gitea.Gitea)
					pair += "+" + second.Spec.Repository + "/" + second.Spec.Branch
				}
				found = append(found, pair)
				assert.Equal(t, "test-token", s.Spec.Token)
				assert.Equal(t, "oauth2", s.Spec.Username)
				assert.Equal(t, server.URL+"/updatecli/"+s.Spec.Repository+".git", s.GetURL())
				assert.Equal(t, "Central update", p.Actions["default"].Title)
				assert.IsType(t, &pullrequest.Gitea{}, p.Actions["default"].Handler)
				assert.Equal(t, "yaml", p.Targets["version"].Config.Kind)
				assert.Equal(t, "version", p.Targets["version"].Config.SourceID)
				report, err := json.Marshal(p.Report)
				require.NoError(t, err)
				assert.NotContains(t, string(report), "test-token")
			}
			assert.ElementsMatch(t, tt.wantPairs, found)
			for _, c := range e.configurations {
				assert.Equal(t, filepath.Dir(filename), c.BaseDir())
				assert.NotEmpty(t, c.ManifestID())
				assert.Equal(t, e.configurations[0].ManifestID(), c.ManifestID())
			}
		})
	}
}
