package jobworker

import (
	"io"
	"os"
	"time"
)

// tailReader wraps the normal io.ReadCloser with an implementation that sleeps for a specified pollInterval
// before retrying to Read
type tailReader struct {
	io.ReadCloser
	pollInterval time.Duration
}

// Read calls the normal io.ReadCloser and checks for an io.EOF error and skips returning, to sleep instead
func (t tailReader) Read(b []byte) (int, error) {
	for {
		n, err := t.ReadCloser.Read(b)
		if n > 0 {
			return n, nil
		} else if err != io.EOF {
			return n, err
		}
		time.Sleep(t.pollInterval)
	}
}

// newTailReader opens a file by path, sets the read offset to the start and returns a wrapped tailReader to the file
func newTailReader(fileName string, pollInterval time.Duration) (tailReader, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return tailReader{}, err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return tailReader{}, err
	}
	return tailReader{f, pollInterval}, nil
}
