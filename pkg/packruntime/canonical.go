package packruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// CanonicalJSON serializes v to JCS-style canonical JSON (RFC 8785):
//   - UTF-8 bytes
//   - object keys sorted lexicographically by their UTF-16 code units
//   - no insignificant whitespace
//   - integers written as integers, floats normalized via strconv shortest form
//
// We re-encode through interface{} after a first round-trip so all map keys
// are surfaced as plain strings regardless of the source struct's tag order.
func CanonicalJSON(v interface{}) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonical: encode: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var any interface{}
	if err := dec.Decode(&any); err != nil {
		return nil, fmt.Errorf("canonical: decode: %w", err)
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, any); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v interface{}) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		writeCanonicalString(buf, x)
	case json.Number:
		writeCanonicalNumber(buf, string(x))
	case float64:
		writeCanonicalNumber(buf, strconv.FormatFloat(x, 'g', -1, 64))
	case []interface{}:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return canonicalKeyLess(keys[i], keys[j]) })
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalString(buf, k)
			buf.WriteByte(':')
			if err := writeCanonical(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("canonical: unsupported type %T", v)
	}
	return nil
}

// canonicalKeyLess compares two strings by their UTF-16 code unit sequence,
// which is what RFC 8785 specifies for object key ordering.
func canonicalKeyLess(a, b string) bool {
	ar := []rune(a)
	br := []rune(b)
	au := runesToUTF16(ar)
	bu := runesToUTF16(br)
	for i := 0; i < len(au) && i < len(bu); i++ {
		if au[i] != bu[i] {
			return au[i] < bu[i]
		}
	}
	return len(au) < len(bu)
}

func runesToUTF16(rs []rune) []uint16 {
	out := make([]uint16, 0, len(rs))
	for _, r := range rs {
		if r < 0x10000 {
			out = append(out, uint16(r))
			continue
		}
		r -= 0x10000
		out = append(out, uint16(0xD800+((r>>10)&0x3FF)))
		out = append(out, uint16(0xDC00+(r&0x3FF)))
	}
	return out
}

func writeCanonicalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
				continue
			}
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
}

func writeCanonicalNumber(buf *bytes.Buffer, s string) {
	// JSON allows arbitrary-precision numbers; Go's standard parser fits them
	// into float64. For canonical output, integers stay as integers; everything
	// else round-trips through strconv shortest.
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		buf.WriteString(strconv.FormatInt(i, 10))
		return
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		buf.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		return
	}
	buf.WriteString(s)
}
