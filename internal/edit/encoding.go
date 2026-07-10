package edit

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// fileEncoding identifies the byte encoding a file was decoded from, so
// the edited content is written back exactly the same way, byte order
// mark included.
type fileEncoding int

// Encodings the engine can read and write. Detection is by byte order
// mark; BOM-less content must be valid UTF-8.
const (
	encUTF8 fileEncoding = iota
	encUTF8BOM
	encUTF16LE
	encUTF16BE
)

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// decode converts raw file bytes into text, detecting the encoding and
// refusing binary content — the engine only edits text files.
func decode(data []byte) (string, fileEncoding, error) {
	switch {
	case bytes.HasPrefix(data, bomUTF8):
		text, err := decodeUTF8(data[len(bomUTF8):])
		return text, encUTF8BOM, err
	case bytes.HasPrefix(data, bomUTF16LE):
		text, err := decodeUTF16(data[len(bomUTF16LE):], true)
		return text, encUTF16LE, err
	case bytes.HasPrefix(data, bomUTF16BE):
		text, err := decodeUTF16(data[len(bomUTF16BE):], false)
		return text, encUTF16BE, err
	default:
		text, err := decodeUTF8(data)
		return text, encUTF8, err
	}
}

// encode converts edited text back into the bytes of its original
// encoding, restoring the byte order mark the file had.
func encode(text string, enc fileEncoding) []byte {
	switch enc {
	case encUTF8BOM:
		return append(append([]byte{}, bomUTF8...), text...)
	case encUTF16LE, encUTF16BE:
		units := utf16.Encode([]rune(text))
		out := make([]byte, 2, 2+2*len(units))
		if enc == encUTF16LE {
			copy(out, bomUTF16LE)
		} else {
			copy(out, bomUTF16BE)
		}
		for _, u := range units {
			if enc == encUTF16LE {
				out = append(out, byte(u), byte(u>>8))
			} else {
				out = append(out, byte(u>>8), byte(u))
			}
		}
		return out
	default:
		return []byte(text)
	}
}

// decodeUTF8 validates BOM-less bytes as UTF-8 text.
func decodeUTF8(data []byte) (string, error) {
	if bytes.IndexByte(data, 0) >= 0 {
		return "", types.NewError(types.ErrorKindValidation,
			"edit: file appears to be binary and cannot be edited as text")
	}
	if !utf8.Valid(data) {
		return "", types.NewError(types.ErrorKindValidation,
			"edit: file is not valid UTF-8; only UTF-8 and UTF-16 (with BOM) files can be edited")
	}
	return string(data), nil
}

// decodeUTF16 decodes byte-order-mark-stripped UTF-16 bytes.
func decodeUTF16(data []byte, littleEndian bool) (string, error) {
	if len(data)%2 != 0 {
		return "", types.NewError(types.ErrorKindValidation,
			"edit: file has a UTF-16 byte order mark but an odd byte length")
	}
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		if littleEndian {
			units = append(units, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	text := string(utf16.Decode(units))
	if strings.ContainsRune(text, 0) {
		return "", types.NewError(types.ErrorKindValidation,
			"edit: file appears to be binary and cannot be edited as text")
	}
	if strings.ContainsRune(text, utf8.RuneError) {
		return "", types.NewError(types.ErrorKindValidation, fmt.Sprintf(
			"edit: file is not valid UTF-16 (%d code units)", len(units)))
	}
	return text, nil
}
