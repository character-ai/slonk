package store

import (
	"encoding/binary"
	"regexp"

	"github.com/uber/kraken/lib/store/metadata"
)

var _taskPidSuffix = "_task_pid"

func init() {
	metadata.Register(regexp.MustCompile(_taskPidSuffix), &taskPidMetadataFactory{})
}

type taskPidMetadataFactory struct{}

func (f taskPidMetadataFactory) Create(suffix string) metadata.Metadata {
	return &TaskPidMetadata{}
}

type TaskPidMetadata struct {
	data int
}

// NewRepresentation creates a new Serialization from string representation.
func NewTaskPidMetadata(pid int) *TaskPidMetadata {
	return &TaskPidMetadata{data: pid}
}

// GetSuffix returns the metadata suffix.
func (s *TaskPidMetadata) GetSuffix() string {
	return _taskPidSuffix
}

// Movable is true.
func (s *TaskPidMetadata) Movable() bool {
	return true
}

// Serialize converts data to bytes.
func (s *TaskPidMetadata) Serialize() ([]byte, error) {
	b := make([]byte, 8)
	binary.PutVarint(b, int64(s.data))
	return b, nil
}

// Deserialize loads bytes into data.
func (s *TaskPidMetadata) Deserialize(b []byte) error {
	i, _ := binary.Varint(b)
	s.data = int(i)
	return nil
}
