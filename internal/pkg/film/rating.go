package film

import (
	"math"
	"sort"
)

const (
	precision       = 1000
	publicPrecision = 10
)

type (
	Rate  float64
	Items []Item
)

func (f Items) SortKinopoisk() {
	sort.Slice(f, func(i, j int) bool {
		a := f[i].RatingKinopoisk
		b := f[j].RatingKinopoisk
		if a == b {
			return f[i].Title < f[j].Title
		}
		return a < b
	})
}

func (f Items) SortIMDB() {
	sort.Slice(f, func(i, j int) bool {
		a := f[i].RatingImdb
		b := f[j].RatingImdb
		if a == b {
			return f[i].Title < f[j].Title
		}
		return a < b
	})
}

func (f *Item) Average() Rate {
	var rate Rate
	for _, v := range f.Scores {
		rate += Rate(v)
	}
	rate /= Rate(len(f.Scores))
	return rate
}

func (f Items) SortAverage() {
	sort.Slice(f, func(i, j int) bool {
		a := f[i].Average().round()
		b := f[j].Average().round()
		if a == b {
			return f[i].Title < f[j].Title
		}
		return a < b
	})
}

func (f *Item) Sum() Rate {
	var rate Rate
	for _, v := range f.Scores {
		rate += Rate(v)
	}
	return rate
}

func (f Items) SortSum() {
	sort.Slice(f, func(i, j int) bool {
		a := f[i].Sum().round()
		b := f[j].Sum().round()
		if a == b {
			return f[i].Title < f[j].Title
		}
		return a < b
	})
}

// Halva = abs(Average) * Sum
func (f *Item) Halva() Rate {
	return Rate(math.Abs(float64(f.Average()))) * f.Sum()
}

func (f Items) SortHalva() {
	sort.Slice(f, func(i, j int) bool {
		a := f[i].Halva().round()
		b := f[j].Halva().round()
		if a == b {
			return f[i].Title < f[j].Title
		}
		return a < b
	})
}

func (r Rate) Round() Rate {
	return Rate(math.Round(float64(r)*publicPrecision) / publicPrecision)
}

func (r Rate) round() Rate {
	return Rate(math.Round(float64(r)*precision) / precision)
}
