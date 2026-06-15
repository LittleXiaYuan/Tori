package gateway

import "yunque-agent/internal/fileparse"

// File preview/analysis parsing now lives in the shared internal/fileparse
// package so capability packs (e.g. the files pack) can reuse it without
// depending on the gateway monolith. These thin wrappers keep the existing
// gateway call sites (upload analysis) and tests stable.

// FileParseResult is the gateway-facing alias for a parsed file preview.
type FileParseResult = fileparse.Result

// TryParseFile extracts a text preview from upload bytes based on filename extension.
func TryParseFile(filename string, data []byte) string {
	return fileparse.Parse(filename, data).Preview
}

// TryParseFileResult extracts a text preview plus user-facing parse metadata.
func TryParseFileResult(filename string, data []byte) FileParseResult {
	return fileparse.Parse(filename, data)
}

func fileParseMetadata(result FileParseResult, previewLimit int) map[string]any {
	return fileparse.Metadata(result, previewLimit)
}
