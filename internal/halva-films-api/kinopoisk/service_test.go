package kinopoisk

import "testing"

func TestKinopoisk_ExtractID(t *testing.T) {
	id := "1209850"
	testCases := []string{
		"https://www.kinopoisk.ru/series/1209850/",
		"https://www.kinopoisk.ru/film/1209850/",
		"https://www.kinopoisk.ru/series/1209850/?utm_referrer=www.google.com",
		"https://www.kinopoisk.ru/film/1209850/?utm_referrer=www.google.com",
	}

	kp := kinopoisk{}
	for i := range testCases {
		got := kp.ExtractID(testCases[i])
		if got != id {
			t.Errorf("expected: %s, got: %s", id, got)
		}
	}
}
