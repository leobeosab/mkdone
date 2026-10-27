// Package scanner discovers Markdown files and parses them concurrently.
package scanner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/ryan/mkdone/internal/model"
	"github.com/ryan/mkdone/internal/parser"
)

// Options controls scanning behavior.
type Options struct {
	// Workers is the number of files parsed in parallel. It defaults to the
	// number of logical CPUs and is capped at the number of files.
	Workers int
}

// Result contains parsed tasks and non-fatal file read warnings.
type Result struct {
	Tasks    []model.Task
	Warnings []error
}

// Scan finds Markdown files under root and parses them using a worker pool.
func Scan(ctx context.Context, root string, options Options) (Result, error) {
	files, err := FindFiles(ctx, root)
	if err != nil {
		return Result{}, err
	}

	workers := options.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(files) {
		workers = len(files)
	}

	type fileResult struct {
		tasks []model.Task
		err   error
	}
	results := make([]fileResult, len(files))
	if len(files) > 0 {
		jobs := make(chan int)
		go func() {
			defer close(jobs)
			for i := range files {
				select {
				case jobs <- i:
				case <-ctx.Done():
					return
				}
			}
		}()

		var wait sync.WaitGroup
		wait.Add(workers)
		for range workers {
			go func() {
				defer wait.Done()
				for index := range jobs {
					if ctx.Err() != nil {
						return
					}
					data, err := os.ReadFile(files[index])
					if err != nil {
						results[index].err = fmt.Errorf("%s: %w", files[index], err)
						continue
					}
					results[index].tasks = parser.Parse(data, files[index])
				}
			}()
		}
		wait.Wait()
	}

	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	result := Result{
		Tasks:    make([]model.Task, 0),
		Warnings: make([]error, 0),
	}
	for _, file := range results {
		if file.err != nil {
			result.Warnings = append(result.Warnings, file.err)
		}
		result.Tasks = append(result.Tasks, file.tasks...)
	}

	sort.SliceStable(result.Tasks, func(i, j int) bool {
		if result.Tasks[i].Source != result.Tasks[j].Source {
			return result.Tasks[i].Source < result.Tasks[j].Source
		}
		if result.Tasks[i].Line != result.Tasks[j].Line {
			return result.Tasks[i].Line < result.Tasks[j].Line
		}
		return result.Tasks[i].Content < result.Tasks[j].Content
	})

	return result, nil
}

// FindFiles returns Markdown files under root. It uses ripgrep when available
// so discovery is fast and handles large trees well; otherwise it falls back
// to filepath.WalkDir.
func FindFiles(ctx context.Context, root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{root}, nil
	}

	rgPath, lookupErr := exec.LookPath("rg")
	if lookupErr == nil {
		files, err := findWithRipgrep(ctx, rgPath, root)
		if err == nil {
			return files, nil
		}
		if !errors.Is(err, exec.ErrNotFound) {
			return nil, err
		}
	}

	return walkFiles(root)
}

func findWithRipgrep(ctx context.Context, rgPath, root string) ([]string, error) {
	args := []string{
		"--files",
		"--null",
		"--hidden",
		"--no-ignore",
		"--iglob", "*.md",
		"--iglob", "*.markdown",
		"--iglob", "!.git/**",
		"--iglob", "!**/.git/**",
		"--",
		root,
	}
	command := exec.CommandContext(ctx, rgPath, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return []string{}, nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return nil, err
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("ripgrep: %s", message)
	}

	parts := strings.Split(stdout.String(), "\x00")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			files = append(files, part)
		}
	}
	sort.Strings(files)
	return files, nil
}

func walkFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md", ".markdown":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
