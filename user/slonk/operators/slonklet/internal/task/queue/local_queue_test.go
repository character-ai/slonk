package queue

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	"your-org.com/slonklet/internal/task/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEnqueueTask(t *testing.T) {
	taskStateDir, err := os.MkdirTemp("/tmp", "test_task_state_*")
	assert.NoError(t, err)
	defer os.RemoveAll(taskStateDir)
	taskLogDir, err := os.MkdirTemp("/tmp", "test_task_log_*")
	assert.NoError(t, err)
	defer os.RemoveAll(taskLogDir)

	logger := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	localQueue, err := NewLocalQueue(taskStateDir, taskLogDir, logger)
	if err != nil {
		t.Fatalf("Failed to create local queue: %s\n", err)
	}

	taskID := core.TaskID(uuid.New())
	task, err := localQueue.EnqueueShellCommandTask(taskID, "echo hello world")
	assert.NoError(t, err)
	task, err = localQueue.GetTask(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, taskID, task.ID)
	assert.Equal(t, core.Queued, task.State)

	processed, err := localQueue.processOneTask(context.Background())
	assert.NoError(t, err)
	assert.True(t, processed)
	task, err = localQueue.GetTask(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, taskID, task.ID)
	assert.Equal(t, core.Succeeded, task.State)

	stdoud, err := localQueue.GetTaskStdoutReader(taskID)
	assert.NoError(t, err)
	output, err := io.ReadAll(stdoud)
	assert.NoError(t, err)
	assert.Equal(t, "hello world\n", string(output))
}
