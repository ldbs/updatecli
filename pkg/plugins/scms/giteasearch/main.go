package giteasearch

import (
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/updatecli/updatecli/pkg/plugins/resources/gitea/client"
)

const (
	DefaultRepositoryLimit = 10
	ErrOwnerEmpty          = "owner is required for giteasearch SCM"
	Kind                   = "giteasearch"
)

// GiteaSearch searches repositories belonging to a Gitea or Forgejo owner.
type GiteaSearch struct {
	spec   Spec
	limit  int
	client client.SDKClient
}

// New creates a GiteaSearch from its configuration.
func New(s interface{}) (*GiteaSearch, error) {
	var spec Spec

	if err := mapstructure.Decode(s, &spec); err != nil {
		return nil, err
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if err := spec.Sanitize(); err != nil {
		return nil, err
	}
	spec.Owner = strings.TrimSpace(spec.Owner)
	spec.Search = strings.TrimSpace(spec.Search)
	if spec.Branch == "" {
		spec.Branch = "^main$"
	}
	limit := DefaultRepositoryLimit
	if spec.Limit != nil {
		limit = *spec.Limit
	}
	c, err := client.NewSDKClient(spec.Spec)
	if err != nil {
		return nil, fmt.Errorf("creating Gitea client: %w", err)
	}
	return &GiteaSearch{spec: spec, limit: limit, client: c}, nil
}
