package gen1

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// ActionEntry is one action of a Gen1 device as GET /settings/actions reports it.
type ActionEntry struct {
	Index   int      `json:"index"`
	URLs    []string `json:"urls"`
	Enabled bool     `json:"enabled"`
}

// encodeForm is url.Values.Encode with one difference: the brackets of a key stay literal, as the
// device's reference writes urls[]=. Every value is percent-encoded as before, and no other Gen1
// form key holds a bracket, so no other form changes.
func encodeForm(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	brackets := strings.NewReplacer("%5B", "[", "%5D", "]")
	var body strings.Builder
	for _, key := range keys {
		escaped := brackets.Replace(url.QueryEscape(key))
		for _, value := range values[key] {
			if body.Len() > 0 {
				body.WriteByte('&')
			}
			body.WriteString(escaped + "=" + url.QueryEscape(value))
		}
	}
	return body.String()
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
