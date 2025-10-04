package queue

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"your-org.com/slonklet/internal/task/core"
	"your-org.com/slonklet/internal/task/entry"
	"your-org.com/slonklet/internal/task/store"
	"github.com/google/uuid"
)

type LocalQueue struct {
	stateStore *store.MultiStateStore
	taskMap    map[core.TaskID]*core.Task
	taskLogDir string
	logger     *log.Logger
}

func NewLocalQueue(taskStateDir string, taskLogDir string, logger *log.Logger) (*LocalQueue, error) {
	// Recover tasks from the state store.
	store, err := store.NewMultiStateStore(taskStateDir, core.AllStates)
	if err != nil {
		return nil, fmt.Errorf("init multi state store: %s", err)
	}
	tasks, err := store.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %s", err)
	}
	taskMap := make(map[core.TaskID]*core.Task)
	for _, task := range tasks {
		taskMap[task.ID] = task
	}

	// TODO (yiran): continue tasks in running state? For now, just mark them as unknown.
	// TODO (yiran): remove finished tasks that are too old.
	for _, task := range taskMap {
		if task.State == core.Running {
			if err := store.UpdateTaskState(task.ID, core.Running, core.Unknown); err != nil {
				// Log and continue
				continue
			}
		}
	}

	// Setup log directory.
	if _, err := os.Stat(taskLogDir); os.IsNotExist(err) {
		err := os.MkdirAll(taskLogDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("mkdir log dir: %s", err)
		}
	}
	files, err := os.ReadDir(taskLogDir)
	if err != nil {
		return nil, fmt.Errorf("read log dir: %s", err)
	}
	for _, file := range files {
		name := filepath.Base(file.Name())
		ext := filepath.Ext(file.Name())
		taskIDStr := name[:len(name)-len(ext)]
		taskID, err := uuid.Parse(taskIDStr)
		if err != nil {
			// Skip non-uuid files.
			continue
		}

		if _, ok := taskMap[core.TaskID(taskID)]; !ok {
			err := os.Remove(filepath.Join(taskLogDir, file.Name()))
			if err != nil {
				// Log and continue
				continue
			}
		}
	}

	return &LocalQueue{
		stateStore: store,
		taskMap:    taskMap,
		taskLogDir: taskLogDir,
		logger:     logger,
	}, nil
}

func (q *LocalQueue) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if processed, err := q.processOneTask(ctx); err != nil {
				// TODO (yiran): log error.
			} else if !processed {
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (q *LocalQueue) EnqueueShellCommandTask(taskID core.TaskID, command string) (*core.Task, error) {
	entry := entry.NewShellCommandTaskEntryFromString(command)
	task, err := q.stateStore.CreateQueuedTask(taskID, core.ShellCommandType, entry)
	if err != nil {
		return nil, fmt.Errorf("create queued task: %s", err)
	}
	q.taskMap[taskID] = task

	return task, nil
}

func (q *LocalQueue) ListTasks() ([]*core.Task, error) {
	tasks, err := q.stateStore.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %s", err)
	}
	return tasks, nil
}

func (q *LocalQueue) GetTask(id core.TaskID) (*core.Task, error) {
	spec, err := q.stateStore.GetTask(id)
	if err != nil && err == os.ErrNotExist {
		return nil, os.ErrNotExist
	} else if err != nil {
		return nil, fmt.Errorf("get task spec: %s", err)
	}
	return spec, nil
}

func (q *LocalQueue) GetTaskStdoutReader(id core.TaskID) (io.ReadCloser, error) {
	stdoutPath := filepath.Join(q.taskLogDir, uuid.UUID(id).String()+".stdout")
	reader, err := os.Open(stdoutPath)
	if err != nil && err == os.ErrNotExist {
		return nil, os.ErrNotExist
	} else if err != nil {
		return nil, fmt.Errorf("get task logs: %s", err)
	}
	return reader, nil
}

func (q *LocalQueue) GetTaskStderrReader(id core.TaskID) (io.ReadCloser, error) {
	stderrPath := filepath.Join(q.taskLogDir, uuid.UUID(id).String()+".stderr")
	reader, err := os.Open(stderrPath)
	if err != nil && err == os.ErrNotExist {
		return nil, os.ErrNotExist
	} else if err != nil {
		return nil, fmt.Errorf("get task logs: %s", err)
	}
	return reader, nil
}

func (q *LocalQueue) KillTask(id core.TaskID) error {
	_, err := q.stateStore.GetTask(id)
	if err != nil && err == os.ErrNotExist {
		return os.ErrNotExist
	} else if err != nil {
		return fmt.Errorf("get task spec: %s", err)
	}

	if err := q.stateStore.UpdateTaskState(id, core.Running, core.Killing); err != nil {
		return fmt.Errorf("update task state to killing: %s", err)
	}
	if err := q.taskMap[id].Entry.Kill(); err != nil {
		return fmt.Errorf("kill task: %s", err)
	}
	if err := q.stateStore.UpdateTaskState(id, core.Killing, core.Killed); err != nil {
		return fmt.Errorf("update task state to killed: %s", err)
	}
	return nil
}

func (q *LocalQueue) processOneTask(ctx context.Context) (bool, error) {
	// Find the oldest queued task.
	emptyTaskID := core.TaskID{}
	taskID := emptyTaskID
	oldestTaskTime := time.Now()
	for id, task := range q.taskMap {
		if task.State == core.Queued {
			if taskID == emptyTaskID {
				taskID = id
			} else {
				lastAccessTime, err := q.stateStore.GetTaskLastAccessTime(id, core.Queued)
				if err != nil {
					// TODO (yiran): log error.
					continue
				}
				if lastAccessTime.Before(oldestTaskTime) {
					taskID = id
					oldestTaskTime = lastAccessTime
				}
			}
		}
	}
	if taskID == emptyTaskID {
		return false, nil
	}

	// Start the task.
	task, err := q.stateStore.GetTask(taskID)
	if err != nil {
		return false, fmt.Errorf("get task spec: %s", err)
	}
	done := make(chan bool)

	stdoutPath := filepath.Join(q.taskLogDir, uuid.UUID(taskID).String()+".stdout")
	stdoutWriter, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return false, fmt.Errorf("get stdout writer: %s", err)
	}
	stderrPath := filepath.Join(q.taskLogDir, uuid.UUID(taskID).String()+".stderr")
	stderrWriter, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return false, fmt.Errorf("get stderr writer: %s", err)
	}
	pid, err := task.Entry.Run(ctx, stdoutWriter, stderrWriter, done)
	if err != nil {
		return false, fmt.Errorf("start task: %s", err)
	}
	if err := q.stateStore.SetTaskPid(taskID, core.Queued, pid); err != nil {
		return false, fmt.Errorf("set task pid: %s", err)
	}
	if err := q.stateStore.UpdateTaskState(taskID, core.Queued, core.Running); err != nil {
		return false, fmt.Errorf("update task state to running: %s", err)
	}

	// Wait for the task to finish.
	result := <-done
	if result {
		q.logger.Printf("Task %s succeeded", uuid.UUID(taskID).String())
		if err := q.stateStore.UpdateTaskState(taskID, core.Running, core.Succeeded); err != nil {
			return false, fmt.Errorf("update task state to succeeded: %s", err)
		}
	} else {
		q.logger.Printf("Task %s failed", uuid.UUID(taskID).String())
		if err := q.stateStore.UpdateTaskState(taskID, core.Running, core.Failed); err != nil {
			return false, fmt.Errorf("update task state to failed: %s", err)
		}
	}

	return true, nil
}
