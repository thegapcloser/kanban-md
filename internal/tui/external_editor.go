package tui

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thegapcloser/kanban-md/internal/task"
)

type externalEditorFinishedMsg struct {
	taskID int
	err    error
}

func (b *Board) openSelectedTaskInExternalEditor() (tea.Model, tea.Cmd) {
	selected := b.selectedTask()
	if selected == nil {
		return b, nil
	}

	taskPath, err := task.FindByID(b.cfg.TasksPath(), selected.ID)
	if err != nil {
		b.err = err
		return b, nil
	}

	cmd, err := externalEditorCommand(taskPath)
	if err != nil {
		b.err = err
		return b, nil
	}

	taskID := selected.ID
	return b, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return externalEditorFinishedMsg{taskID: taskID, err: err}
	})
}

func externalEditorCommand(taskPath string) (*exec.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		var err error
		editor, err = exec.LookPath("vi")
		if err != nil {
			return nil, errors.New("set $VISUAL or $EDITOR, or install vi to open tasks externally")
		}
	}

	return exec.Command(editor, taskPath), nil //nolint:gosec,noctx // editor is an intentional user-configured interactive command
}
