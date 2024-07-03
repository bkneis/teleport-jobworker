package jobworker

import (
	"io"
	"os"
	"sync"
	"time"
)

// tailReader wraps io.ReadCloser with an implementation that can Read and Close in separate go routines
// and be configured to poll for changes to a Reader so Read blocks until Close is called
type tailReader struct {
	sync.RWMutex
	f            io.ReadCloser
	pollInterval time.Duration
	mode         OutputMode
}

// Read calls the normal io.ReadCloser and checks for an io.EOF error
// if mode = FollowLogs it skips returning, to sleep instead for the pollInterval
// if mode = DontFollowLogs we return the error
func (t *tailReader) Read(b []byte) (int, error) {
	for {
		t.RLock()
		n, err := t.f.Read(b)
		t.RUnlock()
		if n > 0 {
			return n, nil
		} else if err != io.EOF {
			return n, err
		} else if err == io.EOF && t.mode == DontFollowLogs {
			return n, err
		}
		time.Sleep(t.pollInterval)
	}
}

// Close closes the underlying io.ReadCloser
func (t *tailReader) Close() error {
	t.Lock()
	defer t.Unlock()
	return t.f.Close()
}

// newTailReader opens a file by path, sets the read offset to the start and returns a wrapped tailReader to the file
func newTailReader(fileName string, pollInterval time.Duration, mode OutputMode) (*tailReader, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return &tailReader{}, err
	}
	return &tailReader{f: f, pollInterval: pollInterval, mode: mode}, nil
}
