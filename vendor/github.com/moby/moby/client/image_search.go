package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/registry"
)

// ImageSearch makes the docker host search by a term in a remote registry.
// The list of results is not sorted in any fashion.
func (cli *Client) ImageSearch(ctx context.Context, term string, options registry.SearchOptions) ([]registry.SearchResult, error) {
	var results []registry.SearchResult
	query := url.Values{}
	query.Set("term", term)
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}

	if options.Filters.Len() > 0 {
		filterJSON, err := filters.ToJSON(options.Filters)
		if err != nil {
			return results, err
		}
		query.Set("filters", filterJSON)
	}

	resp, err := cli.tryImageSearch(ctx, query, options.RegistryAuth)
	defer ensureReaderClosed(resp)
	if cerrdefs.IsUnauthorized(err) && options.PrivilegeFunc != nil {
		newAuthHeader, privilegeErr := options.PrivilegeFunc(ctx)
		if privilegeErr != nil {
			return results, privilegeErr
		}
		resp, err = cli.tryImageSearch(ctx, query, newAuthHeader)
	}
	if err != nil {
		return results, err
	}

	err = json.NewDecoder(resp.Body).Decode(&results)
	return results, err
}

func (cli *Client) tryImageSearch(ctx context.Context, query url.Values, registryAuth string) (*http.Response, error) {
	return cli.get(ctx, "/images/search", query, http.Header{
		registry.AuthHeader: {registryAuth},
	})
}
