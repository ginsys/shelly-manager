package gen1

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ActionEntry is one action of a Gen1 device as GET /settings/actions reports it.
type ActionEntry struct {
	Index   int      `json:"index"`
	URLs    []string `json:"urls"`
	Enabled bool     `json:"enabled"`
}

// SetActionForm writes one action with POST /settings/actions. It sends the index and the name
// always, enabled only when it is not nil, and the URLs only when urls is not nil: one urls[]
// parameter per URL, the form the device reads, and a single empty urls[] for an empty list, which
// is how the device clears the list. A key that was not asked for is left as the device holds it.
func (c *Client) SetActionForm(ctx context.Context, index int, name string, enabled *bool, urls []string) error {
	endpoint := fmt.Sprintf("http://%s/settings/actions", c.ip)
	form := url.Values{}
	form.Set("index", strconv.Itoa(index))
	form.Set("name", name)
	if enabled != nil {
		form.Set("enabled", strconv.FormatBool(*enabled))
	}
	if urls != nil {
		if len(urls) == 0 {
			urls = []string{""}
		}
		form["urls[]"] = urls
	}
	return c.postValues(ctx, endpoint, form)
}

// GetActions reads GET /settings/actions as the entries the device holds, keyed by action name.
func (c *Client) GetActions(ctx context.Context) (map[string][]ActionEntry, error) {
	endpoint := fmt.Sprintf("http://%s/settings/actions", c.ip)
	var result struct {
		Actions map[string][]ActionEntry `json:"actions"`
	}
	if err := c.getJSON(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	return result.Actions, nil
}
