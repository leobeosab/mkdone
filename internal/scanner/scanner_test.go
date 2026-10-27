package scanner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n")
	writeTestFile(t, filepath.Join(root, ".hidden.md"), "# Hidden\n")
	writeTestFile(t, filepath.Join(root, "ignored.md"), "# Ignored\n")
	writeTestFile(t, filepath.Join(root, "sub", "b.markdown"), "# B\n")
	writeTestFile(t, filepath.Join(root, "sub", "upper.MD"), "# Upper\n")
	writeTestFile(t, filepath.Join(root, "not.txt"), "not markdown\n")
	writeTestFile(t, filepath.Join(root, ".git", "inside.md"), "# Git\n")
	writeTestFile(t, filepath.Join(root, ".gitignore"), "ignored.md\n")

	files, err := FindFiles(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, ".hidden.md"),
		filepath.Join(root, "a.md"),
		filepath.Join(root, "ignored.md"),
		filepath.Join(root, "sub", "b.markdown"),
		filepath.Join(root, "sub", "upper.MD"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %#v\nwant   %#v", files, want)
	}
}

func TestFindFilesSingleFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "task.txt")
	writeTestFile(t, path, "- [ ] A task\n")

	files, err := FindFiles(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{path}) {
		t.Errorf("files = %#v, want %#v", files, []string{path})
	}
}

func TestScanSortsTasks(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "b.md"), "- [ ] B1\n- [ ] B2\n")
	writeTestFile(t, filepath.Join(root, "a.md"), "- [ ] A1\n")
	writeTestFile(t, filepath.Join(root, ".git", "skip.md"), "- [ ] Nope\n")

	result, err := Scan(context.Background(), root, Options{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings: %#v", result.Warnings)
	}
	if len(result.Tasks) != 3 {
		t.Fatalf("got %d tasks, want 3: %#v", len(result.Tasks), result.Tasks)
	}
	want := []string{"A1", "B1", "B2"}
	for i, content := range want {
		if result.Tasks[i].Content != content {
			t.Errorf("task %d content = %q, want %q", i, result.Tasks[i].Content, content)
		}
	}
}

func TestWalkFilesSkipsGit(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# A\n")
	writeTestFile(t, filepath.Join(root, "sub", "B.MARKDOWN"), "# B\n")
	writeTestFile(t, filepath.Join(root, ".git", "inside.md"), "# Git\n")
	writeTestFile(t, filepath.Join(root, "x.txt"), "x\n")

	files, err := walkFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "a.md"),
		filepath.Join(root, "sub", "B.MARKDOWN"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("files = %#v, want %#v", files, want)
	}
}
