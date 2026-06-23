package tailreader

import (
	"bytes"
	"io"
	"os"
)

// Cursor reads only the bytes appended to a file since the last call,
// stopping at the last complete newline so a line still being written
// isn't parsed half-finished. Session transcript files (Claude Code,
// Codex, Kimi Code) can grow into the hundreds of MB during a long
// session, so re-reading the whole file on every poll tick isn't viable.
type Cursor struct {
	Path   string
	offset int64
}

func (c *Cursor) ReadNew() ([]byte, error) {
	f, err := os.Open(c.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() < c.offset {
		c.offset = 0
	}

	if _, err := f.Seek(c.offset, io.SeekStart); err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	last := bytes.LastIndexByte(buf, '\n')
	if last < 0 {
		return nil, nil
	}
	complete := buf[:last+1]
	c.offset += int64(len(complete))
	return complete, nil
}
