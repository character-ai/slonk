package store

import (
	"os"
	"testing"

	"your-org.com/slonklet/internal/task/core"
	"your-org.com/slonklet/internal/task/entry"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewMultiStateStore(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "test_*")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	_, err = NewMultiStateStore(dir, core.AllStates)
	assert.Nil(t, err)
}

func TestCreateListAndGetTask(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "test_*")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	store, err := NewMultiStateStore(dir, core.AllStates)
	assert.Nil(t, err)

	// Create.
	id1 := core.TaskID(uuid.New())
	entry1 := entry.NewShellCommandTaskEntryFromString("sleep 10")
	task, err := store.CreateQueuedTask(id1, core.ShellCommandType, entry1)
	assert.Nil(t, err)
	assert.Equal(t, core.Queued, task.State)
	id2 := core.TaskID(uuid.New())
	entry2 := entry.NewShellCommandTaskEntryFromString("sleep 100")
	task, err = store.CreateQueuedTask(id2, core.ShellCommandType, entry2)
	assert.Nil(t, err)
	assert.Equal(t, core.Queued, task.State)

	// List.
	entries, err := store.ListTasks()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(entries))
	idSet := map[core.TaskID]bool{id1: false, id2: false}
	for _, entry := range entries {
		idSet[entry.ID] = true
	}
	assert.True(t, idSet[id1])
	assert.True(t, idSet[id2])

	// Get.
	task, err = store.GetTask(id1)
	assert.Nil(t, err)
	assert.Equal(t, id1, task.ID)
	assert.Equal(t, core.Queued, task.State)
	assert.Equal(t, entry1.Serialize(), task.Entry.Serialize())
}

func TestUpdateTask(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "test_*")
	assert.Nil(t, err)
	defer os.RemoveAll(dir)

	store, err := NewMultiStateStore(dir, core.AllStates)
	assert.Nil(t, err)

	// Create.
	id1 := core.TaskID(uuid.New())
	entry := entry.NewShellCommandTaskEntryFromString("sleep 10")
	task, err := store.CreateQueuedTask(id1, core.ShellCommandType, entry)
	assert.Nil(t, err)
	assert.Equal(t, core.Queued, task.State)

	// Update state.
	err = store.UpdateTaskState(id1, core.Queued, core.Running)
	assert.Nil(t, err)

	// Get.
	task, err = store.GetTask(id1)
	assert.Nil(t, err)
	assert.Equal(t, core.TaskID(id1), task.ID)
	assert.Equal(t, core.Running, task.State)
}
