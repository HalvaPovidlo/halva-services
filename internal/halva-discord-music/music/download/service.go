package download

import (
	"context"
	"fmt"
	"os"

	psong "github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

const (
	removeLimit   = 11
	defaultFormat = ".m4a"
)

var ErrServiceUnknown = fmt.Errorf("service unknown")

type Request struct {
	ID      string
	Source  string
	Service psong.ServiceType
}

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

func (s *service) Download(ctx context.Context, request *Request) (source string, err error) {
	switch request.Service {
	case psong.ServiceYoutube:
		possibleSource := s.pwd + s.youtube.outDirPrefix() + request.ID + defaultFormat
		if _, ok := s.counter[possibleSource]; ok {
			source = possibleSource
			break
		}

		source, err = s.youtube.download(ctx, request.Source)
		if err != nil {
			return "", fmt.Errorf("youtube download: %+w", err)
		}
	case psong.ServiceVK:
		return "", ErrServiceUnknown
	default:
		return "", ErrServiceUnknown
	}

	s.counter[source]++
	return source, nil
}

func (s *service) Delete(request *Request) error {
	if _, ok := s.counter[request.Source]; ok {
		s.counter[request.Source]--
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
