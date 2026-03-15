package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	query := "用一句话介绍Go语言"
	if len(os.Args) > 1 {
		query = strings.Join(os.Args[1:], " ")
	}

	body := fmt.Sprintf(`{"messages":[{"role":"user","content":"%s"}]}`, query)
	req, err := http.NewRequest("POST", "http://localhost:9090/v1/chat/stream", bytes.NewBufferString(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "request error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "tori_default_key_2024")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connection error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error body: %s\n", string(body))
		os.Exit(1)
	}
	fmt.Println("--- SSE Events ---")

	scanner := bufio.NewScanner(resp.Body)
	eventType := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if eventType == "delta" {
				fmt.Print(data)
			} else {
				fmt.Printf("\n[%s] %s\n", eventType, data)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "\nread error: %v\n", err)
	}
	fmt.Println("\n--- Done ---")
}
