package entry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShellCommandEntrySerialization(t *testing.T) {
	entry := NewShellCommandTaskEntryFromString("echo hello world")
	representation := entry.Serialize()
	another := NewShellCommandTaskEntryFromString("test")
	err := another.Deserialize(representation)
	assert.Nil(t, err)
	assert.Equal(t, entry.command, another.command)
}

func TestShellCommandEntryRun(t *testing.T) {
	stdoutFile, err := os.CreateTemp("/tmp", "test_stdout_*")
	assert.Nil(t, err)
	defer os.Remove(stdoutFile.Name())
	stderrFile, err := os.CreateTemp("/tmp", "test_stderr_*")
	assert.Nil(t, err)
	defer os.Remove(stderrFile.Name())

	entry := NewShellCommandTaskEntryFromString("echo hello world")
	done := make(chan bool)
	_, err = entry.Run(context.Background(), stdoutFile, stderrFile, done)
	assert.Nil(t, err)
	select {
	case ok := <-done:
		assert.True(t, ok)
	case <-time.After(3 * time.Second):
		t.Error("Timeout: did not receive message from 'done' channel within 3 seconds")
	}

	stdoutFileContent, err := os.ReadFile(stdoutFile.Name())
	assert.Nil(t, err)
	assert.Equal(t, "hello world\n", string(stdoutFileContent))
}

func TestShellCommandEntryRunAndKill(t *testing.T) {
	stdoutFile, err := os.CreateTemp("/tmp", "test_stdout_*")
	assert.Nil(t, err)
	defer os.Remove(stdoutFile.Name())
	stderrFile, err := os.CreateTemp("/tmp", "test_stderr_*")
	assert.Nil(t, err)
	defer os.Remove(stderrFile.Name())

	entry := NewShellCommandTaskEntryFromString("sleep 10")
	done := make(chan bool)
	_, err = entry.Run(context.Background(), stdoutFile, stderrFile, done)
	assert.Nil(t, err)
	err = entry.Kill()
	assert.Nil(t, err)
	select {
	case ok := <-done:
		assert.False(t, ok)
	case <-time.After(3 * time.Second):
		t.Error("Timeout: did not receive message from 'done' channel within 3 seconds")
	}
}

func TestShellCommandEntryRunAndCancel(t *testing.T) {
	stdoutFile, err := os.CreateTemp("/tmp", "test_stdout_*")
	assert.Nil(t, err)
	defer os.Remove(stdoutFile.Name())
	stderrFile, err := os.CreateTemp("/tmp", "test_stderr_*")
	assert.Nil(t, err)
	defer os.Remove(stderrFile.Name())

	entry := NewShellCommandTaskEntryFromString("sleep 10")
	done := make(chan bool)
	ctx, cancel := context.WithCancel(context.Background())
	_, err = entry.Run(ctx, stdoutFile, stderrFile, done)
	assert.Nil(t, err)
	cancel()

	select {
	case ok := <-done:
		assert.False(t, ok)
	case <-time.After(3 * time.Second):
		t.Error("Timeout: did not receive message from 'done' channel within 3 seconds")
	}
}
