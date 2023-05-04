package film

import (
	"time"

	pcache "github.com/patrickmn/go-cache"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

type cache struct {
	*pcache.Cache
}

func NewCache(defaultExpiration, cleanupInterval time.Duration) *cache {
	return &cache{
		Cache: pcache.New(defaultExpiration, cleanupInterval),
	}
}

func (c *cache) SetFilm(item *film.Item) {
	if item != nil {
		c.SetDefault(item.ID, *item)
	}
}

func (c *cache) GetFilm(id string) (*film.Item, bool) {
	v, ok := c.Get(id)
	if !ok {
		return nil, ok
	}
	if f, ok := v.(film.Item); ok {
		return &f, ok
	}
	return nil, false
}

func (c *cache) SetFilms(items film.Items) {
	for i := range items {
		c.SetFilm(&items[i])
	}
}

func (c *cache) AllFilms() film.Items {
	items := c.Items()
	result := make(film.Items, 0, len(items))
	for _, v := range items {
		if f, ok := v.Object.(film.Item); ok {
			result = append(result, f)
		}
	}
	return result
}
