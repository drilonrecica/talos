// SPDX-License-Identifier: AGPL-3.0-only
package docker

import (
	"context"
	"github.com/drilonrecica/talos/internal/dockerapi"
	"sync"
)

type Metadata struct {
	ID, Name, Image, Created, State, Health string
	Labels                                  map[string]string
	Networks                                []string
	Mounts                                  []dockerapi.Mount
}
type Cache struct {
	mu     sync.RWMutex
	values map[string]Metadata
}

func NewCache() *Cache { return &Cache{values: map[string]Metadata{}} }
func (c *Cache) Discover(ctx context.Context, client dockerapi.Client) error {
	list, e := client.List(ctx)
	if e != nil {
		return e
	}
	next := map[string]Metadata{}
	for _, item := range list {
		v, e := client.Inspect(ctx, item.ID)
		if e != nil {
			return e
		}
		next[item.ID] = Metadata{ID: v.ID, Name: v.Name, Image: v.Image, Created: v.Created, State: v.State, Health: v.Health, Labels: copyLabels(v.Labels), Networks: append([]string(nil), v.Networks...), Mounts: append([]dockerapi.Mount(nil), v.Mounts...)}
	}
	c.mu.Lock()
	c.values = next
	c.mu.Unlock()
	return nil
}
func (c *Cache) Get(id string) (Metadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[id]
	return v, ok
}
func copyLabels(v map[string]string) map[string]string {
	o := map[string]string{}
	for k, x := range v {
		o[k] = x
	}
	return o
}
