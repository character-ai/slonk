package entry

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"

	"your-org.com/slonklet/internal/task/core"
)

func init() {
	core.TaskTypeMap[core.ShellCommandType] = NewShellCommandTaskEntry
}

type ShellCommandTaskEntry struct {
	sync.Mutex

	command    string
	commandObj *exec.Cmd
}

func NewShellCommandTaskEntry() core.TaskEntry {
	return &ShellCommandTaskEntry{}
}

func NewShellCommandTaskEntryFromString(command string) *ShellCommandTaskEntry {
	return &ShellCommandTaskEntry{
		command: command,
	}
}

func (e *ShellCommandTaskEntry) GetType() core.TaskType {
	return core.ShellCommandType
}

func (e *ShellCommandTaskEntry) Serialize() []byte {
	return []byte(e.command)
}

func (e *ShellCommandTaskEntry) Deserialize(serialization []byte) error {
	e.Lock()
	defer e.Unlock()

	e.command = string(serialization)
	return nil
}

func (e *ShellCommandTaskEntry) Run(
	ctx context.Context, stdoutWriter io.WriteCloser, stderrWriter io.WriteCloser, done chan bool,
) (int, error) {
	e.Lock()
	if e.commandObj != nil && e.commandObj.Process != nil {
		e.Unlock()
		return -1, os.ErrExist
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", e.command)
	e.commandObj = cmd
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	defer stdoutWriter.Close()
	defer stderrWriter.Close()

	if err := cmd.Start(); err != nil {
		e.Unlock()
		return -1, err
	}
	e.Unlock()

	pid := cmd.Process.Pid

	go func() {
		err := cmd.Wait()
		done <- err == nil
	}()

	return pid, nil
}

func (e *ShellCommandTaskEntry) Kill() error {
	e.Lock()
	defer e.Unlock()

	if e.commandObj == nil || e.commandObj.Process == nil {
		return os.ErrNotExist
	}

	if err := e.commandObj.Process.Kill(); err != nil {
		return err
	}

	e.commandObj = nil
	return nil
}
