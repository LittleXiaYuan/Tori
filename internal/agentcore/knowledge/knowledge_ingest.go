package knowledge

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// IngestText ingests raw text content.
func (s *Store) IngestText(name, content string) (*Source, error) {
	if content == "" {
		return nil, fmt.Errorf("knowledge: empty content")
	}
	src := s.newSource(name, SourceText)
	chunks := splitText(content, s.chunkSize)
	s.addChunks(src, chunks, nil)
	return src, nil
}

// IngestStructured ingests a knowledge entry with a trigger condition.
func (s *Store) IngestStructured(name, trigger, content string) (*Source, error) {
	if content == "" {
		return nil, fmt.Errorf("knowledge: empty content")
	}
	src := s.newSource(name, SourceText)
	src.Trigger = trigger
	enriched := content
	if trigger != "" {
		enriched = "[使用时机: " + trigger + "]\n" + content
	}
	chunks := splitText(enriched, s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"trigger": trigger})
	return src, nil
}

// UpdateSource updates a knowledge source's metadata.
func (s *Store) UpdateSource(id, name, trigger, content string) (*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found *Source
	for _, src := range s.sources {
		if src.ID == id {
			found = src
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("knowledge: source %q not found", id)
	}

	if name != "" {
		found.Name = name
	}
	found.Trigger = trigger

	if content != "" {
		newChunks := s.chunks[:0]
		for _, c := range s.chunks {
			if c.SourceID != id {
				newChunks = append(newChunks, c)
			}
		}
		s.chunks = newChunks

		enriched := content
		if trigger != "" {
			enriched = "[使用时机: " + trigger + "]\n" + content
		}
		chunks := splitText(enriched, s.chunkSize)
		for i, txt := range chunks {
			s.chunks = append(s.chunks, Chunk{
				ID:       fmt.Sprintf("%s-chunk-%d", id, i),
				SourceID: id,
				Content:  txt,
				Metadata: map[string]string{"trigger": trigger},
				Index:    i,
			})
		}
		found.ChunkCount = len(chunks)
		found.ChunkSize = s.chunkSize
		s.bm25Version++
		if s.semCache != nil {
			s.semCache.Invalidate()
		}
	}

	s.persistKV()
	return found, nil
}

// IngestURL ingests text content fetched from a URL.
func (s *Store) IngestURL(name, sourceURL, content string) (*Source, error) {
	if content == "" {
		return nil, fmt.Errorf("knowledge: empty content")
	}
	if name == "" {
		name = sourceURL
	}
	src := s.newSource(name, SourceURL)
	src.Path = sourceURL
	chunks := splitText(content, s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"url": sourceURL})
	return src, nil
}

// IngestDirectory ingests a local repository or code directory as a single source.
func (s *Store) IngestDirectory(root string, maxFiles int) (*Source, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("knowledge: stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("knowledge: path is not a directory")
	}
	if maxFiles <= 0 {
		maxFiles = 200
	}
	if maxFiles > 1000 {
		maxFiles = 1000
	}

	name := filepath.Base(root)
	src := s.newSource(name, SourceRepo)
	src.Path = root

	prepared := make([]PreparedChunk, 0, maxFiles)
	count := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipRepoDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if count >= maxFiles {
			return io.EOF
		}
		if shouldSkipRepoFile(path, d.Name()) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || len(data) == 0 || len(data) > 512<<10 {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		language := detectRepoLanguage(path)
		chunks := splitRepoContent(filepath.ToSlash(rel), language, string(data), s.chunkSize)
		for _, chunk := range chunks {
			prepared = append(prepared, PreparedChunk{
				Content: chunk,
				Metadata: map[string]string{
					"file": filepath.ToSlash(rel),
					"lang": language,
					"root": filepath.ToSlash(filepath.Clean(root)),
				},
			})
		}
		count++
		return nil
	})
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("knowledge: walk directory: %w", err)
	}
	if len(prepared) == 0 {
		return nil, fmt.Errorf("knowledge: no supported files found")
	}
	s.addPreparedChunks(src, prepared)
	return src, nil
}

// IngestFile ingests a text file (.txt, .md).
func (s *Store) IngestFile(path string) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read file: %w", err)
	}
	name := filepath.Base(path)
	src := s.newSource(name, SourceFile)
	src.Path = path
	chunks := splitText(string(data), s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"file": path})
	return src, nil
}

// IngestCSV ingests a CSV file, treating each row as a chunk.
func (s *Store) IngestCSV(path string) (*Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: open csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("knowledge: csv headers: %w", err)
	}

	name := filepath.Base(path)
	src := s.newSource(name, SourceCSV)
	src.Path = path

	var chunks []string
	var skippedRows int
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skippedRows++
			continue
		}
		var parts []string
		for i, h := range headers {
			if i < len(record) {
				parts = append(parts, h+": "+record[i])
			}
		}
		chunks = append(chunks, strings.Join(parts, " | "))
	}

	if skippedRows > 0 {
		slog.Warn("knowledge: csv rows skipped due to parse errors", "file", path, "skipped", skippedRows)
	}
	s.addChunks(src, chunks, map[string]string{"file": path, "format": "csv"})
	return src, nil
}

// IngestJSON ingests a JSON file (array of objects or single object).
func (s *Store) IngestJSON(path string) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read json: %w", err)
	}

	name := filepath.Base(path)
	src := s.newSource(name, SourceJSON)
	src.Path = path

	var chunks []string

	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		for _, obj := range arr {
			b, _ := json.Marshal(obj)
			chunks = append(chunks, string(b))
		}
	} else {
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err == nil {
			for k, v := range obj {
				b, _ := json.Marshal(v)
				chunks = append(chunks, fmt.Sprintf("%s: %s", k, string(b)))
			}
		} else {
			chunks = splitText(string(data), s.chunkSize)
		}
	}

	s.addChunks(src, chunks, map[string]string{"file": path, "format": "json"})
	return src, nil
}

// IngestPDF ingests a PDF file (extracts text lines from binary).
func (s *Store) IngestPDF(path string) (*Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knowledge: read pdf: %w", err)
	}

	name := filepath.Base(path)
	src := s.newSource(name, SourcePDF)
	src.Path = path

	text := extractReadableText(data)
	if text == "" {
		return nil, fmt.Errorf("knowledge: no text extracted from PDF")
	}

	chunks := splitText(text, s.chunkSize)
	s.addChunks(src, chunks, map[string]string{"file": path, "format": "pdf"})
	return src, nil
}
