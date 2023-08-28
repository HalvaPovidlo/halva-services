package download

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"

	"github.com/HalvaPovidlo/halva-services/internal/pkg/song"
)

type youtube struct {
	outputDir string
}

func NewYouTube(outputDir string) *youtube {
	return &youtube{
		outputDir: outputDir,
	}
}

func (y *youtube) download(ctx context.Context, id string) (string, error) {
	loader := exec.CommandContext(ctx, "./yt-dlp",
		"-f", "ba[ext=m4a][abr<200]",
		"-q",
		"--print", "after_move:filepath",
		"-o", y.outDirPrefix()+string(song.ServiceYoutube)+"_%(id)s.%(ext)s",
		id)

	output, err := loader.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", errors.Wrapf(exitErr, "execute ytdlp: %s", string(exitErr.Stderr))
		}
		return "", errors.Wrap(err, "execute ytdlp")
	}
	source := strings.TrimSuffix(string(output), "\n")
	return source, nil
}

func (y *youtube) outDirPrefix() string {
	return y.outputDir + string(os.PathSeparator)
}
