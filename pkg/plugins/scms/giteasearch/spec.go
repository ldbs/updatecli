package giteasearch

import (
	"errors"
	"strings"

	"github.com/updatecli/updatecli/pkg/plugins/resources/gitea/client"
	"github.com/updatecli/updatecli/pkg/plugins/scms/git/commit"
	"github.com/updatecli/updatecli/pkg/plugins/scms/git/sign"
)

/*
"giteasearch" discovers repositories owned by a Gitea or Forgejo user or organization.
It generates one "gitea" scm for each matching branch of each repository.
Discovery requires both "read:user" and "read:repository" permissions for scoped tokens.
*/
type Spec struct {
	client.Spec `yaml:",inline,omitempty" mapstructure:",squash"`
	// "owner" defines the user or organization whose repositories are searched.
	//
	// remark:
	//   * only repositories accessible to the configured user are returned.
	//   * archived repositories, mirrors and empty repositories are skipped.
	//   * repositories without permission to read their code are skipped.
	//
	Owner string `yaml:",omitempty" jsonschema:"required"`
	// "search" filters repository names by a case-insensitive substring.
	//
	// default:
	//   empty, which selects all repositories owned by "owner".
	//
	Search string `yaml:",omitempty"`
	// "branch" defines a regular expression matching the branches to work on.
	//
	// default:
	//   ^main$
	//
	// remark:
	//   * one Gitea scm is generated for each matching branch of each discovered repository.
	//   * sources, conditions and targets use a working branch based on the matching branch,
	//     named "updatecli_<branch>_<pipelineid>" by default. An existing working branch may be reused.
	//   * set "workingbranch" to false to use the matching branch directly.
	//   * branch names containing "{{" are rejected because pipeline configurations are rendered as Go templates.
	//
	// example:
	//   * branch: ^(main|release/.*)$
	//
	Branch string `yaml:",omitempty"`
	// "limit" defines the maximum number of Gitea scm generated.
	//
	// default:
	//   10
	//
	// remark:
	//   * nonpositive values mean no limit.
	//   * the limit counts matching repository branches, not repositories.
	//
	Limit *int `yaml:",omitempty"`
	// "commitmessage" defines the settings used to generate commit messages.
	//
	// remark:
	//   * the settings apply to every target using this scm.
	//
	CommitMessage commit.Commit `yaml:",omitempty"`
	// "directory" defines the local path where the git repository is cloned.
	//
	// default:
	//   a directory under the Updatecli temporary directory, such as
	//   "/tmp/updatecli/gitea/<owner>/<repository>" on Linux.
	//
	// remark:
	//   * keep the default value unless you have a good reason to change it,
	//     as Updatecli may delete the directory after a pipeline run.
	//   * only set when a single repository is selected after applying "limit".
	//     Selecting multiple repositories with an explicit directory returns an error.
	//
	Directory string `yaml:",omitempty"`
	// "depth" defines the depth used when cloning the git repository.
	//
	// default:
	//   empty, which means a full clone.
	//
	// remark:
	//   * a value greater than 0 creates a shallow clone, so Updatecli cannot see the full git history.
	//     Pushing changes may then fail, in which case setting "force" to true may be needed.
	//   * a negative value is rejected.
	//
	// example:
	//   * depth: 1
	//
	Depth *int `yaml:",omitempty"`
	// "singlebranch" defines whether Updatecli clones and fetches only the matching branch,
	// instead of every branch, tag and other reference of the remote.
	//
	// default:
	//   false
	//
	// remark:
	//   * each repository must have at most one selected branch after applying "limit".
	//   * enabling it can make operations much faster on repositories with many branches, tags
	//     or other references, because Updatecli skips the fetch that mirrors every remote reference.
	//   * in some edge cases, Updatecli may then miss a working branch that was already pushed,
	//     and open a duplicate pull request.
	//
	SingleBranch *bool `yaml:",omitempty"`
	// "email" defines the email address used to author commits.
	//
	// default:
	//   updatecli-bot@updatecli.io
	//
	Email string `yaml:",omitempty"`
	// "user" defines the name used to author commits.
	//
	// default:
	//   updatecli-bot
	//
	User string `yaml:",omitempty"`
	// "force" defines whether Updatecli runs `git push --force` when pushing changes.
	//
	// default:
	//   true
	//
	// remark:
	//   * when true, Updatecli also recreates the working branches that diverged from their base branch.
	//   * when "workingbranch" is false and "force" is not set, each generated Gitea scm returns an error,
	//     to avoid force pushing to "branch" by mistake. Set "force" explicitly to confirm the behavior.
	//
	Force *bool `yaml:",omitempty"`
	// "gpg" defines the GPG key and passphrase used to sign commits.
	//
	GPG sign.GPGSpec `yaml:",omitempty"`
	// "workingbranch" defines whether Updatecli pushes changes to a temporary working branch
	// based on "branch", instead of pushing to "branch" directly.
	//
	// default:
	//   true
	//
	WorkingBranch *bool `yaml:",omitempty"`
	// "workingbranchprefix" defines the prefix of the working branch name.
	//
	// default:
	//   updatecli
	//
	// remark:
	//   * the working branch name joins the prefix, the target branch and the pipeline ID,
	//     separated by "workingbranchseparator".
	//   * when set to an empty string, the name starts with the separator, for example "_main_<pipelineid>".
	//
	WorkingBranchPrefix *string `yaml:",omitempty"`
	// "workingbranchseparator" defines the separator between the parts of the working branch name.
	//
	// default:
	//   _
	//
	WorkingBranchSeparator *string `yaml:",omitempty"`
	// "submodules" defines whether Updatecli clones the git submodules of the repository.
	//
	// default:
	//   true
	//
	Submodules *bool `yaml:",omitempty"`
}

// Validate validates the search settings.
func (s Spec) Validate() error {
	if strings.TrimSpace(s.Owner) == "" {
		return errors.New(ErrOwnerEmpty)
	}
	return nil
}
