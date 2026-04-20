// Package jsonutil collects small helpers for dealing with the
// "JSON-wrapped-in-markdown-or-prose" shape that LLM outputs reliably
// produce. It replaces nine near-duplicate extractJSON implementations
// that used to live in memory/, selfheal/, localbrain/, skillgrow/,
// reflect/, iterate/, curiosity/, world/, handlers_missions/, and
// plugins/education/, each with subtly different corner cases.
//
// The helpers are deliberately small and allocation-light — they run on
// every LLM round trip that expects structured output, so perf matters.
package jsonutil

import (
	"encoding/json"
	"strings"
)

// stripFences removes the most common markdown code-fence wrappers that
// LLMs add around JSON payloads — ```json ... ``` and ``` ... ``` — and
// returns the inner contents after trimming whitespace. A malformed
// fence (open without close) still has the opener trimmed so downstream
// parsing sees clean JSON instead of a stray "```json".
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	// Prefer the explicit ```json marker — more specific match wins so we
	// don't accidentally chop a valid { that starts with ``` later on.
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+len("```json"):]
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+len("```"):]
	}
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// Extract returns the first syntactically balanced JSON object or array
// found in s (after stripping markdown code fences). It prefers whichever
// opener appears first between '{' and '['. When nothing is found it
// returns s unchanged so the caller's json.Unmarshal produces a stable
// parse error rather than a silent "{}". For strict shapes where a
// specific root is required, use ExtractObject or ExtractArray.
//
// "Balanced" here is a byte-level depth count; it does NOT understand
// strings or escapes. That is intentional — LLMs very rarely produce a
// JSON payload whose outermost structure has a string containing an
// unbalanced brace, and full parsing would force a lex pass on every
// call. The fallback behaviour (return s[start:]) gives the decoder a
// chance to report a real parse error when the assumption fails.
func Extract(s string) string {
	s = stripFences(s)
	if s == "" {
		return s
	}

	startObj := strings.IndexByte(s, '{')
	startArr := strings.IndexByte(s, '[')
	var start int
	var openCh, closeCh byte
	switch {
	case startObj < 0 && startArr < 0:
		return s
	case startObj < 0:
		start, openCh, closeCh = startArr, '[', ']'
	case startArr < 0:
		start, openCh, closeCh = startObj, '{', '}'
	case startObj < startArr:
		start, openCh, closeCh = startObj, '{', '}'
	default:
		start, openCh, closeCh = startArr, '[', ']'
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case openCh:
			depth++
		case closeCh:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	// Unbalanced — hand back everything from the opener so the caller can
	// choose to retry, log, or fail-parse on a deterministic slice.
	return s[start:]
}

// ExtractObject returns the first balanced { ... } block in s. If none
// exists it returns "{}" so downstream json.Unmarshal into a struct
// produces a zero value instead of an error. Prefer Extract when you
// can accept either an object or an array; use this when the caller's
// type is specifically an object.
func ExtractObject(s string) string {
	s = stripFences(s)
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "{}"
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "{}"
}

// ExtractArray returns the first balanced [ ... ] block in s. If none
// exists it returns "[]" so downstream json.Unmarshal into a slice
// produces an empty slice. This matches the behaviour of the former
// memory/conflict.extractJSONArray helper.
func ExtractArray(s string) string {
	s = stripFences(s)
	start := strings.IndexByte(s, '[')
	if start < 0 {
		return "[]"
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "[]"
}

// Unmarshal decodes a noisy LLM output string into target. It tries, in
// order:
//   1. Direct decode of the trimmed input (fast path for well-behaved
//      models that return pure JSON).
//   2. Extract + decode (handles ```json fences, leading prose, etc).
//   3. A last-ditch decode of the original raw string so the returned
//      error carries the most useful context for the caller's log.
//
// The target argument must be a non-nil pointer per encoding/json rules;
// passing nil or a non-pointer produces the usual json.Unmarshal error.
func Unmarshal(raw string, target any) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), target); err == nil {
			return nil
		}
	}
	extracted := Extract(raw)
	if err := json.Unmarshal([]byte(extracted), target); err == nil {
		return nil
	}
	// Surface the original error for debuggability — ignoring the error
	// from the Extract pass keeps the message tied to the input the user
	// actually sent, not an internal rewrite.
	return json.Unmarshal([]byte(raw), target)
}
