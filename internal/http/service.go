// Package http provides HTTP client functionality for Wirexa.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	maxResponseBody = 10 * 1024 * 1024 // 10MB
	requestTimeout  = 30 * time.Second
	itemTypeFolder  = "folder"
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// HttpService manages HTTP requests and collections.
type HttpService struct {
	ctx         context.Context
	mu          sync.RWMutex
	client      *http.Client
	collections map[string]*Collection
	storageDir  string
}

// NewHTTPService creates a new HttpService instance.
func NewHTTPService() *HttpService {
	return &HttpService{
		client:      &http.Client{Timeout: requestTimeout},
		collections: make(map[string]*Collection),
	}
}

// SetContext stores the Wails runtime context and loads collections.
func (s *HttpService) SetContext(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()

	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	s.storageDir = filepath.Join(configDir, "Wirexa", "collections")
	os.MkdirAll(s.storageDir, 0o755) //nolint:errcheck,gosec // best-effort directory creation

	s.loadCollections()
}

// SendRequest executes an HTTP request and returns the response.
func (s *HttpService) SendRequest(req HttpRequest) HttpResponse {
	if !validMethods[req.Method] {
		return HttpResponse{Error: fmt.Sprintf("invalid HTTP method: %s", req.Method)}
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return HttpResponse{Error: fmt.Sprintf("invalid URL: %v", err)}
	}

	// Build query parameters
	q := parsedURL.Query()
	for _, p := range req.Params {
		if p.Enabled && p.Key != "" {
			q.Add(p.Key, p.Value)
		}
	}
	parsedURL.RawQuery = q.Encode()

	// Build body
	var bodyReader io.Reader
	contentType := ""
	switch req.Body.Type {
	case "json":
		bodyReader = strings.NewReader(req.Body.Content)
		contentType = "application/json"
	case "text":
		bodyReader = strings.NewReader(req.Body.Content)
		contentType = "text/plain"
	case "form-urlencoded":
		bodyReader = strings.NewReader(req.Body.Content)
		contentType = "application/x-www-form-urlencoded"
	case "form-data":
		bodyReader = strings.NewReader(req.Body.Content)
		// Note: multipart/form-data requires a boundary; the raw content is
		// passed through as-is so the caller must supply a correct Content-Type
		// header with boundary when using form-data.
	}

	s.mu.RLock()
	ctx := s.ctx
	client := s.client
	s.mu.RUnlock()

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, parsedURL.String(), bodyReader)
	if err != nil {
		return HttpResponse{Error: fmt.Sprintf("failed to create request: %v", err)}
	}

	// Set headers
	for _, h := range req.Headers {
		if h.Enabled && h.Key != "" {
			httpReq.Header.Set(h.Key, h.Value)
		}
	}
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	start := time.Now()
	resp, err := client.Do(httpReq)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return HttpResponse{Error: fmt.Sprintf("request failed: %v", err), TimingMs: elapsed}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return HttpResponse{Error: fmt.Sprintf("failed to read response: %v", err), TimingMs: elapsed}
	}

	truncated := false
	if int64(len(body)) > maxResponseBody {
		body = body[:maxResponseBody]
		truncated = true
	}

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	result := HttpResponse{
		StatusCode:  resp.StatusCode,
		StatusText:  resp.Status,
		Headers:     headers,
		Body:        string(body),
		ContentType: resp.Header.Get("Content-Type"),
		Size:        int64(len(body)),
		TimingMs:    elapsed,
	}
	if truncated {
		result.Error = fmt.Sprintf("response body truncated at %d bytes", maxResponseBody)
	}
	return result
}

// GetCollections returns all collections.
func (s *HttpService) GetCollections() []Collection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Collection, 0, len(s.collections))
	for _, c := range s.collections {
		result = append(result, *c)
	}
	return result
}

// CreateCollection creates a new collection with the given name.
func (s *HttpService) CreateCollection(name string) Collection {
	c := Collection{
		ID:    uuid.New().String(),
		Name:  name,
		Items: []TreeItem{},
	}

	s.mu.Lock()
	s.collections[c.ID] = &c
	data, err := marshalCollection(&c)
	s.mu.Unlock()

	if err == nil {
		_ = s.writeCollectionFile(c.ID, data)
	}
	return c
}

// DeleteCollection deletes a collection by ID.
func (s *HttpService) DeleteCollection(id string) error {
	s.mu.Lock()
	if _, ok := s.collections[id]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("collection not found: %s", id)
	}
	delete(s.collections, id)
	s.mu.Unlock()

	if err := os.Remove(filepath.Join(s.storageDir, id+".json")); err != nil {
		return err
	}

	return nil
}

// RenameCollection renames a collection.
func (s *HttpService) RenameCollection(id, name string) error {
	s.mu.Lock()
	c, ok := s.collections[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("collection not found: %s", id)
	}
	c.Name = name
	data, err := marshalCollection(c)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal collection: %w", err)
	}
	return s.writeCollectionFile(id, data)
}

// AddFolder adds a folder to a collection.
func (s *HttpService) AddFolder(collectionID, parentID, name string) (*TreeItem, error) {
	s.mu.Lock()
	c, ok := s.collections[collectionID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("collection not found: %s", collectionID)
	}

	item := TreeItem{
		Type:     itemTypeFolder,
		ID:       uuid.New().String(),
		Name:     name,
		Children: []TreeItem{},
	}

	if parentID == "" {
		c.Items = append(c.Items, item)
	} else if !addToParent(&c.Items, parentID, item) {
		s.mu.Unlock()
		return nil, fmt.Errorf("parent not found: %s", parentID)
	}

	data, err := marshalCollection(c)
	s.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("marshal collection: %w", err)
	}
	if err := s.writeCollectionFile(collectionID, data); err != nil {
		return nil, err
	}
	return &item, nil
}

// AddRequest adds a request to a collection.
func (s *HttpService) AddRequest(collectionID, parentID string, req HttpRequest) (*TreeItem, error) {
	s.mu.Lock()
	c, ok := s.collections[collectionID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("collection not found: %s", collectionID)
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	item := TreeItem{
		Type:    "request",
		ID:      req.ID,
		Name:    req.Name,
		Request: &req,
	}

	if parentID == "" {
		c.Items = append(c.Items, item)
	} else if !addToParent(&c.Items, parentID, item) {
		s.mu.Unlock()
		return nil, fmt.Errorf("parent not found: %s", parentID)
	}

	data, err := marshalCollection(c)
	s.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("marshal collection: %w", err)
	}
	if err := s.writeCollectionFile(collectionID, data); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateRequest updates a request in a collection.
func (s *HttpService) UpdateRequest(collectionID string, req HttpRequest) error {
	s.mu.Lock()
	c, ok := s.collections[collectionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("collection not found: %s", collectionID)
	}

	if !updateRequestInTree(&c.Items, req) {
		s.mu.Unlock()
		return fmt.Errorf("request not found: %s", req.ID)
	}

	data, err := marshalCollection(c)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal collection: %w", err)
	}
	return s.writeCollectionFile(collectionID, data)
}

// RenameItem renames a folder or request in a collection.
func (s *HttpService) RenameItem(collectionID, itemID, name string) error {
	s.mu.Lock()
	c, ok := s.collections[collectionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("collection not found: %s", collectionID)
	}

	if !renameInTree(&c.Items, itemID, name) {
		s.mu.Unlock()
		return fmt.Errorf("item not found: %s", itemID)
	}

	data, err := marshalCollection(c)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal collection: %w", err)
	}
	return s.writeCollectionFile(collectionID, data)
}

// DeleteItem deletes a folder or request from a collection.
func (s *HttpService) DeleteItem(collectionID, itemID string) error {
	s.mu.Lock()
	c, ok := s.collections[collectionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("collection not found: %s", collectionID)
	}

	if !removeFromTree(&c.Items, itemID) {
		s.mu.Unlock()
		return fmt.Errorf("item not found: %s", itemID)
	}

	data, err := marshalCollection(c)
	s.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal collection: %w", err)
	}
	return s.writeCollectionFile(collectionID, data)
}

// Tree helpers

func addToParent(items *[]TreeItem, parentID string, child TreeItem) bool {
	for i := range *items {
		if (*items)[i].ID == parentID && (*items)[i].Type == itemTypeFolder {
			(*items)[i].Children = append((*items)[i].Children, child)
			return true
		}
		if (*items)[i].Type == itemTypeFolder {
			if addToParent(&(*items)[i].Children, parentID, child) {
				return true
			}
		}
	}
	return false
}

func renameInTree(items *[]TreeItem, id, name string) bool {
	for i := range *items {
		if (*items)[i].ID == id {
			(*items)[i].Name = name
			if (*items)[i].Request != nil {
				(*items)[i].Request.Name = name
			}
			return true
		}
		if (*items)[i].Type == itemTypeFolder {
			if renameInTree(&(*items)[i].Children, id, name) {
				return true
			}
		}
	}
	return false
}

func removeFromTree(items *[]TreeItem, id string) bool {
	for i := range *items {
		if (*items)[i].ID == id {
			*items = append((*items)[:i], (*items)[i+1:]...)
			return true
		}
		if (*items)[i].Type == itemTypeFolder {
			if removeFromTree(&(*items)[i].Children, id) {
				return true
			}
		}
	}
	return false
}

func updateRequestInTree(items *[]TreeItem, req HttpRequest) bool {
	for i := range *items {
		if (*items)[i].ID == req.ID && (*items)[i].Type == "request" {
			(*items)[i].Name = req.Name
			(*items)[i].Request = &req
			return true
		}
		if (*items)[i].Type == itemTypeFolder {
			if updateRequestInTree(&(*items)[i].Children, req) {
				return true
			}
		}
	}
	return false
}

// Persistence

// marshalCollection serializes a collection to JSON bytes. Safe to call under lock.
func marshalCollection(c *Collection) ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// writeCollectionFile writes pre-marshaled collection data to disk. Must be called outside lock.
func (s *HttpService) writeCollectionFile(id string, data []byte) error {
	if err := os.WriteFile(filepath.Join(s.storageDir, id+".json"), data, 0o600); err != nil {
		return fmt.Errorf("write collection: %w", err)
	}
	return nil
}

// Shutdown closes the HTTP client's idle connections.
func (s *HttpService) Shutdown() {
	s.client.CloseIdleConnections()
}

func (s *HttpService) loadCollections() {
	entries, err := os.ReadDir(s.storageDir)
	if err != nil {
		return
	}

	loaded := make(map[string]*Collection, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.storageDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("warning: failed to read collection file %s: %v", entry.Name(), err)
			continue
		}

		var c Collection
		if err := json.Unmarshal(data, &c); err != nil {
			log.Printf("warning: failed to parse collection file %s: %v", entry.Name(), err)
			continue
		}

		loaded[c.ID] = &c
	}

	s.mu.Lock()
	maps.Copy(s.collections, loaded)
	s.mu.Unlock()
}
