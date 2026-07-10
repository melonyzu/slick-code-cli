package edit

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// writeAtomic writes data to path without ever exposing a partially
// written file: the bytes go to a temporary file in the same directory
// (so the final rename never crosses a filesystem), are flushed to
// disk, and the temporary file is renamed over the target in one atomic
// step. On any failure the temporary file is removed and the target is
// left exactly as it was.
func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".slickcode-edit-*.tmp")
	if err != nil {
		return types.WrapError(types.ErrorKindInternal,
			"edit: create temporary file in "+dir, err)
	}
	name := tmp.Name()

	fail := func(step string, err error) error {
		tmp.Close()
		os.Remove(name)
		return types.WrapError(types.ErrorKindInternal, "edit: "+step, err)
	}

	if _, err := tmp.Write(data); err != nil {
		return fail("write temporary file", err)
	}
	if err := tmp.Sync(); err != nil {
		return fail("flush temporary file", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fail("set file mode", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return types.WrapError(types.ErrorKindInternal, "edit: close temporary file", err)
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return types.WrapError(types.ErrorKindInternal, "edit: replace "+path, err)
	}
	return nil
}
