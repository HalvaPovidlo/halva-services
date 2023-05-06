package film

import (
	"time"

	pcache "github.com/patrickmn/go-cache"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

type cache struct {
	film *pcache.Cache
	user *pcache.Cache
}

func NewCache(defaultExpiration, cleanupInterval time.Duration) *cache {
	return &cache{
		film: pcache.New(defaultExpiration, cleanupInterval),
		user: pcache.New(defaultExpiration, cleanupInterval),
	}
}

func (c *cache) Set(item *film.Item) {
	if item != nil {
		c.film.SetDefault(item.ID, *item)
	}
}

func (c *cache) Get(id string) (*film.Item, bool) {
	v, ok := c.film.Get(id)
	if !ok {
		return nil, false
	}
	if f, ok := v.(film.Item); ok {
		return &f, true
	}
	return nil, false
}

func (c *cache) SetAll(items film.Items) {
	for i := range items {
		c.Set(&items[i])
	}
}

func (c *cache) All() film.Items {
	items := c.film.Items()
	result := make(film.Items, 0, len(items))
	for _, v := range items {
		if f, ok := v.Object.(film.Item); ok {
			result = append(result, f)
		}
	}
	return result
}

func (c *cache) SetUser(userID string, items film.Items) {
	c.user.SetDefault(userID, items)
}

func (c *cache) User(userID string) (film.Items, bool) {
	v, ok := c.user.Get(userID)
	if !ok {
		return nil, false
	}
	if f, ok := v.(film.Items); ok {
		return f, true
	}
	return nil, false
}
