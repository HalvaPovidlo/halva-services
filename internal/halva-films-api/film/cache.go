package film

import (
	"sync"
	"time"

	pcache "github.com/patrickmn/go-cache"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/film"
)

type userCache map[string]map[string]struct{}

type cache struct {
	film *pcache.Cache // films.Item

	mx   *sync.RWMutex
	user userCache // userID -> filmsID -> struct
}

func NewCache(defaultExpiration, cleanupInterval time.Duration) *cache {
	return &cache{
		film: pcache.New(defaultExpiration, cleanupInterval),
		mx:   &sync.RWMutex{},
		user: make(userCache, 10),
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

func (c *cache) User(userID string) ([]string, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	films, ok := c.user[userID]
	if !ok {
		return nil, false
	}

	res := make([]string, 0, len(films))
	for k, _ := range films {
		res = append(res, k)
	}
	return res, true
}

func (c *cache) SetUser(userID string, filmsID []string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	films, _ := c.user[userID]
	if len(films) == 0 {
		films = make(map[string]struct{}, len(filmsID))
	}
	for i := range filmsID {
		films[filmsID[i]] = struct{}{}
	}

	c.user[userID] = films
}

func (c *cache) UserAdd(userID string, filmID string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	films, _ := c.user[userID]
	if len(films) == 0 {
		films = make(map[string]struct{})
	}

	films[filmID] = struct{}{}
	c.user[userID] = films
}

func (c *cache) UserRemove(userID string, filmID string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	films, _ := c.user[userID]
	if len(films) == 0 {
		films = make(map[string]struct{})
	}

	delete(films, filmID)
}
