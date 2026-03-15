package http

// HttpRequest represents an HTTP request to be sent.
type HttpRequest struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Method  string         `json:"method"`
	URL     string         `json:"url"`
	Headers []KeyValuePair `json:"headers"`
	Params  []KeyValuePair `json:"params"`
	Body    RequestBody    `json:"body"`
}

// KeyValuePair represents a key-value pair with an enabled toggle.
type KeyValuePair struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// RequestBody represents the body of an HTTP request.
type RequestBody struct {
	Type    string `json:"type"` // "none","json","text","form-urlencoded","form-data"
	Content string `json:"content"`
}

// HttpResponse represents the response from an HTTP request.
type HttpResponse struct {
	StatusCode  int               `json:"statusCode"`
	StatusText  string            `json:"statusText"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	ContentType string            `json:"contentType"`
	Size        int64             `json:"size"`
	TimingMs    int64             `json:"timingMs"`
	Error       string            `json:"error,omitempty"`
}

// Collection represents a named group of requests and folders.
type Collection struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Items []TreeItem `json:"items"`
}

// TreeItem represents a folder or request in the collection tree.
type TreeItem struct {
	Type     string       `json:"type"` // "folder" or "request"
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Children []TreeItem   `json:"children,omitempty"`
	Request  *HttpRequest `json:"request,omitempty"`
}
