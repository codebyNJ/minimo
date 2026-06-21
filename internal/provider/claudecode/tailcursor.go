package claudecode

import (
	"bytes"
	"io"
	"os"
)

type tailCursor struct {
	path   string
	offset int64
}

func (c *tailCursor) readNew() ([]byte, error) {
	f, err := os.Open(c.path)
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
