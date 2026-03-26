package knowledge

import "fmt"

// IngestDocxBytes ingests a DOCX file from raw bytes.
// Extracts readable text content from the OOXML format.
func (s *Store) IngestDocxBytes(name string, data []byte) (*Source, error) {
	text := extractReadableText(data) // reuse PDF text extractor for basic extraction
	if text == "" {
		return nil, fmt.Errorf("knowledge: no text extracted from DOCX")
	}
	src := s.newSource(name, SourceFile)
	chunks := splitText(text, s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"format": "docx"})
	return src, nil
}

// IngestXlsxBytes ingests an XLSX file from raw bytes.
// Extracts readable text from the OOXML spreadsheet format.
func (s *Store) IngestXlsxBytes(name string, data []byte) (*Source, error) {
	text := extractReadableText(data) // reuse PDF text extractor for basic extraction
	if text == "" {
		return nil, fmt.Errorf("knowledge: no text extracted from XLSX")
	}
	src := s.newSource(name, SourceFile)
	chunks := splitText(text, s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"format": "xlsx"})
	return src, nil
}

// GetSource returns a source by ID, or nil if not found.
func (s *Store) GetSource(sourceID string) *Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sources[sourceID]
}
