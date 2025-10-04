package store

import (
	"regexp"

	"your-org.com/slonklet/internal/task/core"
	"github.com/uber/kraken/lib/store/metadata"
)

var _taskTypeSuffix = "_task_type"

func init() {
	metadata.Register(regexp.MustCompile(_taskTypeSuffix), &taskTypeMetadataFactory{})
}

type taskTypeMetadataFactory struct{}

func (f taskTypeMetadataFactory) Create(suffix string) metadata.Metadata {
	return &TaskTypeMetadata{}
}

type TaskTypeMetadata struct {
	data core.TaskType
}

// NewRepresentation creates a new Serialization from string representation.
func NewTaskTypeMetadata(taskType core.TaskType) *TaskTypeMetadata {
	return &TaskTypeMetadata{data: taskType}
}

// GetSuffix returns the metadata suffix.
func (s *TaskTypeMetadata) GetSuffix() string {
	return _taskTypeSuffix
}

// Movable is true.
func (s *TaskTypeMetadata) Movable() bool {
	return true
}

// Serialize converts data to bytes.
func (s *TaskTypeMetadata) Serialize() ([]byte, error) {
	b := []byte(s.data)
	return b, nil
}

// Deserialize loads bytes into data.
func (s *TaskTypeMetadata) Deserialize(b []byte) error {
	s.data = core.TaskType(string(b))
	return nil
}
