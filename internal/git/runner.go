package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

const maxGitOutput = 4 * 1024 * 1024

// ManagerParams contains dependencies required to construct a Manager.
type ManagerParams struct {
	// Repository is the working tree the manager operates on.
	Repository Repository

	// Executable is the Git executable path. Empty uses "git" from PATH.
	Executable string

	// Logger receives structured operation events.
	Logger *slog.Logger
}

// Manager executes bounded Git operations for one repository.
type Manager struct {
	repository Repository
	executable string
	logger     *slog.Logger
}

// NewManager returns a repository manager built from params.
func NewManager(params ManagerParams) (*Manager, error) {
	if params.Repository.Root == "" || params.Repository.GitDir == "" {
		return nil, types.NewError(types.ErrorKindValidation, "git: repository root and metadata directory are required")
	}
	if params.Executable == "" {
		params.Executable = "git"
	}
	return &Manager{
		repository: params.Repository,
		executable: params.Executable,
		logger:     gitLogger(params.Logger),
	}, nil
}

// Repository returns the immutable repository metadata owned by the manager.
func (m *Manager) Repository() Repository {
	return m.repository
}

type commandResult struct {
	stdout   string
	stderr   string
	code     int
	err      error
	overflow bool
}

func (m *Manager) run(ctx context.Context, operation string, args ...string) (string, error) {
	result := m.runRaw(ctx, operation, args...)
	if result.err != nil || result.code != 0 || result.overflow {
		return "", classifyCommandError(ctx, operation, result)
	}
	return result.stdout, nil
}

func (m *Manager) runRaw(ctx context.Context, operation string, args ...string) commandResult {
	started := time.Now()
	hooksDir, hooksErr := os.MkdirTemp("", "slickcode-git-hooks-")
	if hooksErr != nil {
		return commandResult{err: hooksErr, code: -1, stderr: "create isolated hooks directory: " + hooksErr.Error()}
	}
	defer os.RemoveAll(hooksDir)
	commandArgs := make([]string, 0, len(args)+4)
	commandArgs = append(commandArgs, "-C", m.repository.Root, "-c", "core.hooksPath="+hooksDir)
	commandArgs = append(commandArgs, args...)
	cmd := exec.CommandContext(ctx, m.executable, commandArgs...)
	cmd.Env = gitEnvironment()
	stdout, stderr := &cappedBuffer{limit: maxGitOutput}, &cappedBuffer{limit: maxGitOutput}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	m.logger.Debug("git operation started", "operation", operation)
	err := cmd.Run()
	result := commandResult{
		stdout: stdout.String(), stderr: stderr.String(), code: exitCode(err), err: err,
		overflow: stdout.overflow || stderr.overflow,
	}
	if err == nil {
		m.logger.Debug("git operation completed", "operation", operation, "duration", time.Since(started))
	} else {
		m.logger.Warn("git operation failed", "operation", operation, "exit_code", result.code,
			"duration", time.Since(started), "error", strings.TrimSpace(result.stderr))
	}
	return result
}

func classifyCommandError(ctx context.Context, operation string, result commandResult) error {
	if err := ctx.Err(); err != nil {
		kind := types.ErrorKindCanceled
		if errors.Is(err, context.DeadlineExceeded) {
			kind = types.ErrorKindTimeout
		}
		return types.WrapError(kind, "git: "+operation, err)
	}
	if result.overflow {
		return types.WrapError(types.ErrorKindValidation,
			fmt.Sprintf("git: %s produced more than %d bytes", operation, maxGitOutput),
			errors.New("git output limit exceeded"))
	}
	message := strings.TrimSpace(result.stderr)
	if message == "" {
		message = strings.TrimSpace(result.stdout)
	}
	if message == "" {
		message = "command failed"
	}
	lower := strings.ToLower(message)
	kind := types.ErrorKindInternal
	switch {
	case strings.Contains(lower, "not a git repository"), strings.Contains(lower, "pathspec"),
		strings.Contains(lower, "unknown revision"), strings.Contains(lower, "not a valid branch name"),
		strings.Contains(lower, "did not match any file"), strings.Contains(lower, "invalid object name"):
		kind = types.ErrorKindValidation
	case strings.Contains(lower, "would be overwritten"), strings.Contains(lower, "local changes"),
		strings.Contains(lower, "unmerged"), strings.Contains(lower, "conflict"),
		strings.Contains(lower, "nothing to commit"), strings.Contains(lower, "already exists"):
		kind = types.ErrorKindConflict
	case strings.Contains(lower, "permission denied"):
		kind = types.ErrorKindPermissionDenied
	}
	return types.WrapError(kind, "git: "+operation+": "+message, result.err)
}

func trimLineEnding(value string) string {
	value = strings.TrimSuffix(value, "\n")
	return strings.TrimSuffix(value, "\r")
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func gitEnvironment() []string {
	return append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat", "LC_ALL=C")
}

type cappedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	if b.buf.Len()+len(data) > b.limit {
		remaining := max(0, b.limit-b.buf.Len())
		if remaining > 0 {
			_, _ = b.buf.Write(data[:remaining])
		}
		b.overflow = true
		return len(data), nil
	}
	return b.buf.Write(data)
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}
