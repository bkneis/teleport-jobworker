package jobworker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

const testFile = "/tmp/tail_reader_test"

func Test_tailReader_follow(t *testing.T) {
	n := 5
	// Create a test file with known contents
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Error(err)
	}
	// Create tail reader and read from checking contents
	reader, err := newTailReader(testFile, 20*time.Millisecond, FollowLogs)
	defer reader.Close()
	// Append some "testing n" to emulate a job logging to STDOUT
	go func(r io.ReadCloser) {
		// Close the tail reader once writing is done
		defer r.Close()
		// Open TEST file in append only
		f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		// Append some logging output
		for i := range n {
			if _, err = f.WriteString(fmt.Sprintf("testing %d\n", i)); err != nil {
				t.Error("failed to write string to test file", err)
				return
			}
		}
	}(reader)

	// TODO in production to prevent the CI from blocking we would use a context with a short timeout
	// that once Done() would close the reader causing the scanner to break on the io.EOF
	// Read the job's "output" written from the goroutine above
	scanner := bufio.NewScanner(reader)
	logs := []string{}
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
		if len(logs) >= n {
			break
		}
	}
	// Assert the logs are as expected
	for i, log := range logs {
		expected := fmt.Sprintf("testing %d", i)
		if log != expected {
			t.Errorf("expected log line to be %s but was actually %s", expected, log)
		}
	}
}

func Test_tailReader_no_follow(t *testing.T) {
	n := 5
	// Create a test file with known contents
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Error(err)
	}
	// Create tail reader and read from checking contents
	reader, err := newTailReader(testFile, 20*time.Millisecond, DontFollowLogs)
	defer reader.Close()
	// Append some "testing n" to emulate a job logging to STDOUT
	f, err := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	// Append some logging output
	for i := range n {
		if _, err = f.WriteString(fmt.Sprintf("testing %d\n", i)); err != nil {
			t.Error("failed to write string to test file", err)
			return
		}
	}

	// Read the job's "output" written from the goroutine above
	scanner := bufio.NewScanner(reader)
	logs := []string{}
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
		if len(logs) >= n {
			break
		}
	}
	// Assert the logs are as expected
	for i, log := range logs {
		expected := fmt.Sprintf("testing %d", i)
		if log != expected {
			t.Errorf("expected log line to be %s but was actually %s", expected, log)
		}
	}
}
