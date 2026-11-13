package giteasearch

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	giteasdk "gitea.dev/sdk"
	giteascm "github.com/updatecli/updatecli/pkg/plugins/scms/gitea"
)

// ScmsGenerator returns a Gitea scm specification for each matching repository branch.
func (g *GiteaSearch) ScmsGenerator(ctx context.Context) ([]giteascm.Spec, error) {
	branchPattern, err := regexp.Compile(g.spec.Branch)
	if err != nil {
		return nil, fmt.Errorf("invalid branch regex %q: %w", g.spec.Branch, err)
	}
	search := strings.ToLower(g.spec.Search)
	var results []giteascm.Spec
	opts := giteasdk.ListReposOptions{ListOptions: giteasdk.ListOptions{Page: 1, PageSize: 50}}
	for {
		// This endpoint accepts both user and organization names on Gitea and Forgejo.
		repos, resp, err := g.client.Repositories.ListUserRepos(ctx, g.spec.Owner, opts)
		if err != nil {
			return nil, fmt.Errorf("listing Gitea repositories for %q: %w", g.spec.Owner, err)
		}
		for _, repo := range repos {
			if repo.Archived || repo.Mirror || repo.Empty || !strings.Contains(strings.ToLower(repo.Name), search) {
				continue
			}
			if repo.Permissions != nil && !repo.Permissions.Pull {
				continue
			}
			selectedBranch := false
			branchOpts := giteasdk.ListRepoBranchesOptions{ListOptions: giteasdk.ListOptions{Page: 1, PageSize: 50}}
			for {
				branches, branchResp, err := g.client.Repositories.ListRepoBranches(ctx, g.spec.Owner, repo.Name, branchOpts)
				if err != nil {
					return nil, fmt.Errorf("listing Gitea branches for %s/%s: %w", g.spec.Owner, repo.Name, err)
				}
				for _, branch := range branches {
					if !branchPattern.MatchString(branch.Name) {
						continue
					}
					if strings.Contains(branch.Name, "{{") {
						return nil, fmt.Errorf("unsupported template branch name %q", branch.Name)
					}
					if g.spec.Directory != "" && len(results) > 0 && !strings.EqualFold(results[0].Repository, repo.Name) {
						return nil, fmt.Errorf("directory requires a single selected repository")
					}
					// Branches of the same repository share a clone.
					if selectedBranch && g.spec.SingleBranch != nil && *g.spec.SingleBranch {
						return nil, fmt.Errorf("singlebranch requires a single selected branch for %s/%s", g.spec.Owner, repo.Name)
					}
					selectedBranch = true
					results = append(results, giteascm.Spec{
						Spec:                   g.spec.Spec,
						Owner:                  g.spec.Owner,
						Repository:             repo.Name,
						Branch:                 branch.Name,
						CommitMessage:          g.spec.CommitMessage,
						Directory:              g.spec.Directory,
						Depth:                  g.spec.Depth,
						SingleBranch:           g.spec.SingleBranch,
						Email:                  g.spec.Email,
						User:                   g.spec.User,
						Force:                  g.spec.Force,
						GPG:                    g.spec.GPG,
						WorkingBranch:          g.spec.WorkingBranch,
						WorkingBranchPrefix:    g.spec.WorkingBranchPrefix,
						WorkingBranchSeparator: g.spec.WorkingBranchSeparator,
						Submodules:             g.spec.Submodules,
					})
					if g.limit > 0 && len(results) >= g.limit {
						return results, nil
					}
				}
				if branchResp.NextPage == 0 {
					break
				}
				branchOpts.Page = branchResp.NextPage
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return results, nil
}
