package firestore

import (
	"math/rand"
	"time"

	pcache "github.com/patrickmn/go-cache"

	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

type cache struct {
	songs *pcache.Cache // songs.Item
}

func NewCache(defaultExpiration, cleanupInterval time.Duration) *cache {
	return &cache{
		songs: pcache.New(defaultExpiration, cleanupInterval),
	}
}

func (c *cache) Set(item *psong.Item) {
	if item != nil {
		c.songs.SetDefault(string(item.ID), *item)
	}
}

func (c *cache) Get(id psong.IDType) (*psong.Item, bool) {
	v, ok := c.songs.Get(string(id))
	if !ok {
		return nil, false
	}
	if s, ok := v.(psong.Item); ok {
		return &s, true
	}
	return nil, false
}

func (c *cache) GetAny(minPlaybacks int64) *psong.Item {
	if c.songs.ItemCount() == 0 {
		return nil
	}

	items := c.songs.Items()
	songs := make([]psong.Item, 0, len(items))
	for _, v := range items {
		if s, ok := v.Object.(psong.Item); ok {
			if s.Count >= minPlaybacks {
				songs = append(songs, s)
			}
		}
	}

	r := rand.New(rand.NewSource(time.Now().Unix()))
	return &songs[r.Intn(len(songs))]
}
