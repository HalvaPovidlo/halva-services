package download

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"

	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

const (
	removeLimit   = 11
	defaultFormat = ".m4a"
)

var ErrServiceUnknown = fmt.Errorf("service unknown")

type service struct {
	youtube *youtube

	counter       map[string]int
	removeCounter int
	pwd           string
}

func New(outputDir string) (*service, error) {
	if err := os.RemoveAll(outputDir); err != nil {
		return nil, fmt.Errorf("os remove all %s: %+w", outputDir, err)
	}
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os getwd: %+w", err)
	}

	return &service{
		youtube: NewYouTube(outputDir),
		counter: make(map[string]int, removeLimit+1),
		pwd:     pwd + string(os.PathSeparator),
	}, nil
}

func (s *service) Download(ctx context.Context, request *psong.Item) (path string, err error) {
	switch request.Service {
	case psong.ServiceYoutube:
		possibleSource := s.pwd + s.youtube.outDirPrefix() + string(request.ID) + defaultFormat
		if _, ok := s.counter[possibleSource]; ok {
			path = possibleSource
			break
		}

		path, err = s.youtube.download(ctx, request.URL)
		if err != nil {
			return "", errors.Wrap(err, "youtube download")
		}
	case psong.ServiceVK:
		return "", ErrServiceUnknown
	default:
		return "", ErrServiceUnknown
	}

	s.counter[path]++
	return path, nil
}

func (s *service) Delete(path string) error {
	if _, ok := s.counter[path]; ok {
		s.counter[path]--
	}
	return s.removeZeroes()
}

func (s *service) removeZeroes() error {
	if s.removeCounter < removeLimit {
		s.removeCounter++
		return nil
	}

	brokenFiles := ""
	s.removeCounter = 0
	for source, counter := range s.counter {
		if counter > 0 {
			return nil
		}
		if err := os.Remove(source); err != nil {
			brokenFiles += source + " - " + err.Error() + ","
		}
		delete(s.counter, source)
	}
	if brokenFiles != "" {
		return fmt.Errorf("os remove files: %s", brokenFiles)
	}
	return nil
}
