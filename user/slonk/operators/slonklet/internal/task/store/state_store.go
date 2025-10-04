package store

import (
	"fmt"
	"os"
	"path"
	"time"

	"your-org.com/slonklet/internal/task/core"
	"github.com/andres-erbsen/clock"
	"github.com/google/uuid"
	"github.com/uber/kraken/lib/store/base"
	"github.com/uber/kraken/lib/store/metadata"
)

type MultiStateStore struct {
	store  base.FileStore
	states map[core.TaskState]*base.FileState
}

func NewMultiStateStore(parentDir string, states []core.TaskState) (*MultiStateStore, error) {
	if err := os.MkdirAll(parentDir, 0775); err != nil {
		return nil, fmt.Errorf("mkdir: %s", err)
	}

	store := base.NewLocalFileStore(clock.New())
	stateMap := make(map[core.TaskState]*base.FileState)

	for _, state := range states {
		dir := path.Join(parentDir, string(state))
		if err := os.MkdirAll(dir, 0775); err != nil {
			return nil, fmt.Errorf("mkdir: %s", err)
		}
		interalState := base.NewFileState(dir)
		stateMap[state] = &interalState
	}

	return &MultiStateStore{store, stateMap}, nil
}

func (s *MultiStateStore) CreateQueuedTask(taskID core.TaskID, taskType core.TaskType, taskEntry core.TaskEntry) (*core.Task, error) {
	internalQueuedState := s.states[core.Queued]

	serialization := taskEntry.Serialize()
	taskIDStr := uuid.UUID(taskID).String()
	if err := s.store.NewFileOp().AcceptState(*internalQueuedState).CreateFile(taskIDStr, *internalQueuedState, int64(len(serialization))); err != nil {
		return nil, fmt.Errorf("create file: %s", err)
	}
	readWriter, err := s.store.NewFileOp().AcceptState(*internalQueuedState).GetFileReadWriter(taskIDStr)
	if err != nil {
		return nil, fmt.Errorf("get read writer: %s", err)
	}
	if _, err := readWriter.Write(serialization); err != nil {
		return nil, fmt.Errorf("write serialization: %s", err)
	}
	if err := readWriter.Close(); err != nil {
		return nil, fmt.Errorf("close read writer: %s", err)
	}

	typeMeta := NewTaskTypeMetadata(taskType)
	if _, err := s.store.NewFileOp().AcceptState(*internalQueuedState).SetFileMetadata(taskIDStr, typeMeta); err != nil {
		return nil, fmt.Errorf("set type metadata: %s", err)
	}
	latMeta := metadata.NewLastAccessTime(time.Now())
	if _, err := s.store.NewFileOp().AcceptState(*internalQueuedState).SetFileMetadata(taskIDStr, latMeta); err != nil {
		return nil, fmt.Errorf("set lat metadata: %s", err)
	}

	return &core.Task{
		ID:    taskID,
		Type:  taskType,
		State: core.Queued,
		Entry: taskEntry,
	}, nil
}

func (s *MultiStateStore) UpdateTaskState(taskID core.TaskID, currentState core.TaskState, targetState core.TaskState) error {
	internalCurrentState := s.states[currentState]
	internalTargetState := s.states[targetState]

	return s.store.NewFileOp().AcceptState(*internalCurrentState).MoveFile(uuid.UUID(taskID).String(), *internalTargetState)
}

func (s *MultiStateStore) ListTasks() ([]*core.Task, error) {
	result := make([]*core.Task, 0)
	for state, internalState := range s.states {
		taskIDs, err := s.store.NewFileOp().AcceptState(*internalState).ListNames()
		if err != nil {
			return nil, fmt.Errorf("list names: %s", err)
		}
		for _, taskIDStr := range taskIDs {
			var typeMeta TaskTypeMetadata
			if err := s.store.NewFileOp().AcceptState(*internalState).GetFileMetadata(taskIDStr, &typeMeta); err != nil {
				return nil, fmt.Errorf("get serialization metadata: %s", err)
			}
			reader, err := s.store.NewFileOp().AcceptState(*internalState).GetFileReader(taskIDStr)
			if err != nil {
				return nil, fmt.Errorf("get serialization reader: %s", err)
			}
			serialization := make([]byte, reader.Size())
			if _, err := reader.Read(serialization); err != nil {
				return nil, fmt.Errorf("read serialization: %s", err)
			}
			entry := core.TaskTypeMap[typeMeta.data]()
			if err := entry.Deserialize(serialization); err != nil {
				return nil, fmt.Errorf("deserialize: %s", err)
			}

			taskID, err := uuid.Parse(taskIDStr)
			if err != nil {
				return nil, fmt.Errorf("parse task id: %s", err)
			}
			result = append(result, &core.Task{
				ID:    core.TaskID(taskID),
				State: state,
				Type:  typeMeta.data,
				Entry: entry,
			})
		}
	}
	return result, nil

}

func (s *MultiStateStore) GetTask(taskID core.TaskID) (*core.Task, error) {
	for state, internalState := range s.states {
		_, err := s.store.NewFileOp().AcceptState(*internalState).GetFileStat(uuid.UUID(taskID).String())
		if err == nil {
			var typeMeta TaskTypeMetadata
			if err := s.store.NewFileOp().AcceptState(*internalState).GetFileMetadata(uuid.UUID(taskID).String(), &typeMeta); err != nil {
				return nil, fmt.Errorf("get serialization metadata: %s", err)
			}
			reader, err := s.store.NewFileOp().AcceptState(*internalState).GetFileReader(uuid.UUID(taskID).String())
			if err != nil {
				return nil, fmt.Errorf("get serialization reader: %s", err)
			}
			serialization := make([]byte, reader.Size())
			if _, err := reader.Read(serialization); err != nil {
				return nil, fmt.Errorf("read serialization: %s", err)
			}
			f, ok := core.TaskTypeMap[typeMeta.data]
			if !ok {
				return nil, fmt.Errorf("unknown task type: %s", typeMeta.data)
			}
			entry := f()
			if err := entry.Deserialize(serialization); err != nil {
				return nil, fmt.Errorf("deserialize: %s", err)
			}
			return &core.Task{
				ID:    taskID,
				State: state,
				Type:  typeMeta.data,
				Entry: entry,
			}, nil
		}
	}

	return nil, os.ErrNotExist
}

func (s *MultiStateStore) SetTaskPid(taskID core.TaskID, currentState core.TaskState, pid int) error {
	internalState := s.states[currentState]
	_, err := s.store.NewFileOp().AcceptState(*internalState).SetFileMetadata(uuid.UUID(taskID).String(), NewTaskPidMetadata(pid))
	if err != nil {
		return fmt.Errorf("set pid metadata: %s", err)
	}
	return nil
}

func (s *MultiStateStore) GetTaskPid(taskID core.TaskID, currentState core.TaskState) (int, error) {
	internalState := s.states[currentState]
	var pidMeta TaskPidMetadata
	if err := s.store.NewFileOp().AcceptState(*internalState).GetFileMetadata(uuid.UUID(taskID).String(), &pidMeta); err != nil {
		return 0, fmt.Errorf("get pid metadata: %s", err)
	}
	return pidMeta.data, nil
}

func (s *MultiStateStore) GetTaskLastAccessTime(taskID core.TaskID, currentState core.TaskState) (time.Time, error) {
	internalState := s.states[currentState]
	var latMeta metadata.LastAccessTime
	if err := s.store.NewFileOp().AcceptState(*internalState).GetFileMetadata(uuid.UUID(taskID).String(), &latMeta); err != nil {
		return time.Time{}, fmt.Errorf("get lat metadata: %s", err)
	}
	return latMeta.Time, nil
}
