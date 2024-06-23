package jobworker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

const TEST_FILE = "/tmp/tail_reader_test"
const TEST_LOGS = "hello world\nhello test\n"
const EXPECTED_LOGS = "hello world\nhello test\ntesting 0\ntesting 1\ntesting 2\n"

func Test_tailReader(t *testing.T) {
	// Create a test file with known contents
	err := os.WriteFile(TEST_FILE, []byte(TEST_LOGS), 0644)
	if err != nil {
		t.Error(err)
	}

	// Create tail reader and read from checking contents
	reader, err := newTailReader(TEST_FILE, 20*time.Millisecond)
	defer reader.Close()

	// Append some "testing n" to emulate a job logging to STDOUT
	go func(r io.ReadCloser) {
		// Close the tail reader once writing is done
		defer r.Close()
		// Open TEST file in append only
		f, err := os.OpenFile(TEST_FILE, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		// Append some logging output
		for i := range 3 {
			if _, err = f.WriteString(fmt.Sprintf("testing %d\n", i)); err != nil {
				t.Error("failed to write string to test file", err)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}(reader)

	// Read the job's "output" written from the goroutine above
	scanner := bufio.NewScanner(reader)
	recievedLogs := ""
	for scanner.Scan() {
		line := scanner.Text()
		recievedLogs += line + "\n"
	}

	// Assert the logs are as expected
	if EXPECTED_LOGS != recievedLogs {
		t.Error("Logs read from tail reader was not the same as ones sent", EXPECTED_LOGS, recievedLogs)
		return
	}
}
