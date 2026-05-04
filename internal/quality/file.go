package quality

import (
	"context"
	"fmt"
	"os"
)

// FileExistsCheck is a binary check: 100 if the file is stat-able, 0 otherwise.
type FileExistsCheck struct {
	Path string
}

func (FileExistsCheck) Name() string { return "file_exists" }

func (f FileExistsCheck) Run(_ context.Context, _ Input) Result {
	if _, err := os.Stat(f.Path); err != nil {
		return Result{Name: "file_exists", Score: 0, Passed: false, Note: fmt.Sprintf("%s missing: %v", f.Path, err)}
	}
	return Result{Name: "file_exists", Score: 100, Passed: true, Note: fmt.Sprintf("%s present", f.Path)}
}
