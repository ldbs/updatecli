package giteasearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	giteasdk "gitea.dev/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/updatecli/updatecli/pkg/plugins/resources/gitea/client"
	"github.com/updatecli/updatecli/pkg/plugins/scms/git/commit"
	"github.com/updatecli/updatecli/pkg/plugins/scms/git/sign"
	giteascm "github.com/updatecli/updatecli/pkg/plugins/scms/gitea"
)

func TestScmsGenerator(t *testing.T) {
	for _, tt := range []struct {
		name         string
		limit        int
		branch       string
		search       string
		directory    string
		emptyPage    bool
		singleBranch bool
		failurePath  string
		wantErr      string
		wantBranches []string
		wantCalls    int
	}{
		{
			name:         "All repository and branch pages",
			branch:       "^(main|release/1)$",
			search:       "APP",
			wantBranches: []string{"app-one/main", "app-one/release/1", "app-two/main"},
			wantCalls:    5,
		},
		{
			name:         "Limit counts matching branches",
			branch:       "^(main|release/1)$",
			directory:    "checkout",
			limit:        2,
			wantBranches: []string{"app-one/main", "app-one/release/1"},
			wantCalls:    3,
		},
		{
			name:         "Limit after a filtered repository page",
			branch:       "^main$",
			search:       "APP-TWO",
			limit:        1,
			wantBranches: []string{"app-two/main"},
			wantCalls:    3,
		},
		{
			name:         "Empty repository page with a next page",
			branch:       "^main$",
			emptyPage:    true,
			wantBranches: []string{"app-one/main", "app-two/main"},
			wantCalls:    6,
		},
		{
			name:         "Matching branch on a later page",
			branch:       "^release/1$",
			limit:        1,
			wantBranches: []string{"app-one/release/1"},
			wantCalls:    3,
		},
		{
			name:         "Single branch clone for each repository",
			branch:       "^main$",
			singleBranch: true,
			wantBranches: []string{"app-one/main", "app-two/main"},
			wantCalls:    5,
		},
		{
			name:         "Single branch clone with a limit",
			branch:       "^(main|release/1)$",
			directory:    "checkout",
			limit:        1,
			singleBranch: true,
			wantBranches: []string{"app-one/main"},
			wantCalls:    2,
		},
		{
			name:         "Single branch clone with multiple selected branches",
			branch:       "^(main|release/1)$",
			singleBranch: true,
			wantErr:      "singlebranch requires a single selected branch",
			wantCalls:    3,
		},
		{
			name:      "Directory shared by different repositories",
			branch:    "^main$",
			directory: "checkout",
			wantErr:   "directory requires a single selected repository",
			wantCalls: 5,
		},
		{
			name:         "Directory with only one matching repository",
			branch:       "^release/1$",
			directory:    "checkout",
			wantBranches: []string{"app-one/release/1"},
			wantCalls:    5,
		},
		{
			name:      "Selected template branch",
			branch:    ".*",
			wantErr:   "unsupported template branch name",
			wantCalls: 2,
		},
		{
			name:         "Limit before a template branch",
			branch:       ".*",
			limit:        1,
			wantBranches: []string{"app-one/main"},
			wantCalls:    2,
		},
		{
			name:      "No matching repository",
			branch:    "^main$",
			search:    "missing",
			wantCalls: 2,
		},
		{
			name:      "No matching branch",
			branch:    "^missing$",
			wantCalls: 5,
		},
		{
			name:    "Invalid branch pattern",
			branch:  "[",
			wantErr: "invalid branch regex",
		},
		{
			name:        "Repository listing error",
			branch:      "^main$",
			failurePath: "/users/updatecli/repos",
			wantErr:     "listing Gitea repositories",
			wantCalls:   1,
		},
		{
			name:        "Branch listing error",
			branch:      "^main$",
			failurePath: "/repos/updatecli/app-one/branches",
			wantErr:     "listing Gitea branches",
			wantCalls:   2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
				calls++
				if r.URL.Path == "/forge/api/v1"+tt.failurePath {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				nextPage := func(page int) {
					w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=%d>; rel="next"`, r.Host, r.URL.Path, page))
				}
				switch r.URL.Path {
				case "/forge/api/v1/users/updatecli/repos":
					if r.URL.Query().Get("page") == "1" {
						nextPage(2)
						_ = json.NewEncoder(w).Encode([]giteasdk.Repository{
							{Name: "archived", Archived: true},
							{Name: "mirror", Mirror: true},
							{Name: "empty", Empty: true},
							{Name: "issues-only", Permissions: &giteasdk.Permission{}},
							{Name: "app-one", Permissions: &giteasdk.Permission{Pull: true}},
						})
					} else if tt.emptyPage && r.URL.Query().Get("page") == "2" {
						nextPage(3)
						_, _ = w.Write([]byte(`[]`))
					} else {
						_ = json.NewEncoder(w).Encode([]giteasdk.Repository{{Name: "app-two", Fork: true}})
					}
				case "/forge/api/v1/repos/updatecli/app-one/branches":
					if r.URL.Query().Get("page") == "1" {
						nextPage(2)
						_ = json.NewEncoder(w).Encode([]giteasdk.Branch{{Name: "main"}, {Name: "develop"}, {Name: "{{.SCMs.default.Handler.Spec.Token}}"}})
					} else {
						_ = json.NewEncoder(w).Encode([]giteasdk.Branch{{Name: "release/1"}})
					}
				case "/forge/api/v1/repos/updatecli/app-two/branches":
					_ = json.NewEncoder(w).Encode([]giteasdk.Branch{{Name: "main"}})
				default:
					t.Errorf("unexpected request: %s", r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			depth, enabled, disabled, prefix, separator := 1, true, false, "bot", "-"
			spec := Spec{
				Spec:                   client.Spec{URL: server.URL + "/forge", Token: "test-token", Username: "bot"},
				Owner:                  "updatecli",
				Branch:                 tt.branch,
				Search:                 tt.search,
				CommitMessage:          commit.Commit{Type: "chore"},
				Directory:              tt.directory,
				Depth:                  &depth,
				SingleBranch:           &tt.singleBranch,
				Email:                  "bot@example.com",
				User:                   "Bot",
				Force:                  &disabled,
				GPG:                    sign.GPGSpec{SigningKey: "test-key", Passphrase: "test-passphrase"},
				WorkingBranch:          &enabled,
				WorkingBranchPrefix:    &prefix,
				WorkingBranchSeparator: &separator,
				Submodules:             &disabled,
			}
			c, err := client.NewSDKClient(spec.Spec)
			require.NoError(t, err)
			g := GiteaSearch{spec: spec, limit: tt.limit, client: c}
			got, err := g.ScmsGenerator(t.Context())
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			var branches []string
			for _, generated := range got {
				branches = append(branches, generated.Repository+"/"+generated.Branch)
				assert.Equal(t, giteascm.Spec{
					Spec:                   spec.Spec,
					Owner:                  "updatecli",
					Repository:             generated.Repository,
					Branch:                 generated.Branch,
					CommitMessage:          spec.CommitMessage,
					Directory:              tt.directory,
					Depth:                  &depth,
					SingleBranch:           &tt.singleBranch,
					Email:                  "bot@example.com",
					User:                   "Bot",
					Force:                  &disabled,
					GPG:                    spec.GPG,
					WorkingBranch:          &enabled,
					WorkingBranchPrefix:    &prefix,
					WorkingBranchSeparator: &separator,
					Submodules:             &disabled,
				}, generated)
			}
			assert.Equal(t, tt.wantBranches, branches)
			assert.Equal(t, tt.wantCalls, calls)
		})
	}
}
