package core

import (
	"context"
	"io"

	"github.com/google/uuid"
)

type TaskID uuid.UUID

type TaskState string

const (
	// Init stage
	Queued TaskState = "queued"

	// Progressing stage
	Starting TaskState = "starting"
	Running  TaskState = "running"
	Killing  TaskState = "killing" // Not used in the current implementation

	// Terminal stage
	Invalid   TaskState = "invalid"
	Succeeded TaskState = "succeeded"
	Failed    TaskState = "failed"
	Killed    TaskState = "killed" // Not used in the current implementation

	Unknown TaskState = "unknown"
)

var AllStates []TaskState = []TaskState{
	Queued,
	Starting,
	Running,
	Killing,
	Invalid,
	Succeeded,
	Failed,
	Killed,
	Unknown,
}

type TaskType string

const (
	ShellCommandType TaskType = "shell_command"
)

var TaskTypeMap map[TaskType]func() TaskEntry

type TaskEntry interface {
	GetType() TaskType

	Serialize() []byte
	Deserialize(representation []byte) error

	Run(ctx context.Context, stdoutWriter io.WriteCloser, stderrWriter io.WriteCloser, done chan bool) (int, error)
	Kill() error
}

type Task struct {
	ID    TaskID
	Type  TaskType
	State TaskState
	Entry TaskEntry
}

func init() {
	TaskTypeMap = map[TaskType]func() TaskEntry{}
}
