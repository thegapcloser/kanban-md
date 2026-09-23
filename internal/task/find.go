package task

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/thegapcloser/kanban-md/internal/clierr"
)

// idPrefixRe matches the numeric ID prefix of a task filename.
var idPrefixRe = regexp.MustCompile(`^(\d+)-`)

const taskFileExt = ".md"

// FindByID scans the tasks directory for a file matching the given ID.
// Returns the full path to the task file.
func FindByID(tasksDir string, id int) (string, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return "", fmt.Errorf("reading tasks directory: %w", err)
	}

	idStr := strconv.Itoa(id)
	path, prefixFallback := findByFilenamePrefix(entries, tasksDir, idStr, id)
	if path != "" {
		return path, nil
	}

	path = findByFrontmatterID(entries, tasksDir, id)
	if path != "" {
		return path, nil
	}
	if prefixFallback != "" {
		return prefixFallback, nil
	}

	return "", clierr.Newf(clierr.TaskNotFound, "task not found: #%d", id).
		WithDetails(map[string]any{"id": id})
}

func findByFilenamePrefix(entries []os.DirEntry, tasksDir, idStr string, id int) (string, string) {
	var prefixFallback string
	for _, entry := range entries {
		name := entry.Name()
		if !isTaskMarkdown(entry) {
			continue
		}
		// Strip leading zeros and check if the numeric prefix matches the ID.
		dash := strings.IndexByte(name, '-')
		if dash < 1 {
			continue
		}
		prefix := strings.TrimLeft(name[:dash], "0")
		if prefix != idStr {
			continue
		}

		path := filepath.Join(tasksDir, name)
		t, readErr := Read(path)
		if readErr != nil {
			if prefixFallback == "" {
				prefixFallback = path
			}
			continue
		}
		if t.ID == id {
			return path, prefixFallback
		}
	}
	return "", prefixFallback
}

func findByFrontmatterID(entries []os.DirEntry, tasksDir string, id int) string {
	for _, entry := range entries {
		if !isTaskMarkdown(entry) {
			continue
		}
		path := filepath.Join(tasksDir, entry.Name())
		t, readErr := Read(path)
		if readErr != nil {
			continue
		}
		if t.ID == id {
			return path
		}
	}
	return ""
}

func isTaskMarkdown(entry os.DirEntry) bool {
	return !entry.IsDir() && strings.HasSuffix(entry.Name(), taskFileExt)
}

// ReadAll reads all task files from the given directory.
func ReadAll(tasksDir string) ([]*Task, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading tasks directory: %w", err)
	}

	var tasks []*Task
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != taskFileExt {
			continue
		}

		path := filepath.Join(tasksDir, entry.Name())
		t, err := Read(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

// ReadWarning describes a file that could not be parsed during lenient reading.
type ReadWarning struct {
	File string // base filename
	Err  error
}

// ReadAllLenient reads all task files, skipping malformed files instead of aborting.
// Successfully parsed tasks are returned along with warnings for files that failed.
func ReadAllLenient(tasksDir string) ([]*Task, []ReadWarning, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading tasks directory: %w", err)
	}

	var tasks []*Task
	var warnings []ReadWarning
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != taskFileExt {
			continue
		}

		path := filepath.Join(tasksDir, entry.Name())
		t, readErr := Read(path)
		if readErr != nil {
			warnings = append(warnings, ReadWarning{File: entry.Name(), Err: readErr})
			continue
		}
		tasks = append(tasks, t)
	}

	return tasks, warnings, nil
}

// ExtractIDFromFilename extracts the numeric ID from a task filename.
func ExtractIDFromFilename(filename string) (int, error) {
	matches := idPrefixRe.FindStringSubmatch(filename)
	if len(matches) < 2 { //nolint:mnd // regex capture group
		return 0, fmt.Errorf("cannot extract ID from filename %q", filename)
	}
	return strconv.Atoi(matches[1])
}
