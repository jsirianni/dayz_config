package logreader

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
)

const MaxArchiveBytes = 64 << 20

// CompressedTail bounds both compressed input and expanded work. The ring keeps
// only the final MaxBytes; incomplete archives are skipped, never presented as
// complete tails. In particular, corrupt/oversized private-key data stays hidden.
func CompressedTail(input io.Reader, lines int) (Excerpt, error) {
	gz, err := gzip.NewReader(io.LimitReader(input, MaxArchiveBytes+1))
	if err != nil {
		return Excerpt{}, err
	}
	defer gz.Close()
	ring := make([]byte, MaxBytes)
	buf := make([]byte, 64<<10)
	total, pos := 0, 0
	limited := io.LimitReader(gz, MaxArchiveBytes+1)
	for {
		n, err := limited.Read(buf)
		total += n
		if total > MaxArchiveBytes {
			return Excerpt{Truncated: true, BytesRead: MaxArchiveBytes}, errors.New("compressed log exceeds 64 MiB decompression limit; excerpt skipped")
		}
		first := min(n, len(ring)-pos)
		copy(ring[pos:], buf[:first])
		copy(ring, buf[first:n])
		pos = (pos + n) % len(ring)
		if err == io.EOF {
			break
		}
		if err != nil {
			return Excerpt{Truncated: true}, err
		}
	}
	var data []byte
	if total <= len(ring) {
		data = ring[:total]
	} else {
		data = append(append([]byte{}, ring[pos:]...), ring[:pos]...)
		// The retained window can begin in the middle of a JSON record.
		if end := bytes.IndexByte(data, '\n'); end >= 0 {
			data = data[end+1:]
		} else {
			data = nil
		}
	}
	out, err := Tail(bytes.NewReader(data), int64(len(data)), lines, false)
	out.Truncated = out.Truncated || total > MaxBytes
	return out, err
}
