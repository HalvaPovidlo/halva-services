package user

import (
	"github.com/HalvaPovidlo/halva-services/internal/pkg/user"
	"time"

	pcache "github.com/patrickmn/go-cache"
)

type cache struct {
	*pcache.Cache
}

func NewCache(defaultExpiration, cleanupInterval time.Duration) *cache {
	return &cache{
		Cache: pcache.New(defaultExpiration, cleanupInterval),
	}
}

func (c *cache) Set(item *user.Item) {
	if item != nil {
		c.SetDefault(item.ID, *item)
	}
}

func (c *cache) Get(id string) (*user.Item, bool) {
	v, ok := c.Cache.Get(id)
	if !ok {
		return nil, false
	}
	if u, ok := v.(user.Item); ok {
		return &u, ok
	}
	return nil, false
}

func (c *cache) All() user.Items {
	items := c.Items()
	result := make(user.Items, 0, len(items))
	for _, v := range items {
		if u, ok := v.Object.(user.Item); ok {
			result = append(result, u)
		}
	}
	return result
}
