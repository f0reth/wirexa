package httpdomain

// HttpRequest は HTTP リクエストを表す。
type HttpRequest struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Method  string         `json:"method"`
	URL     string         `json:"url"`
	Headers []KeyValuePair `json:"headers"`
	Params  []KeyValuePair `json:"params"`
	Body    RequestBody    `json:"body"`
}

// KeyValuePair はヘッダーやパラメータのキーバリューペアを表す。
type KeyValuePair struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// RequestBody はリクエストボディを表す。
type RequestBody struct {
	Type    string `json:"type"` // "none"|"json"|"text"|"form-urlencoded"|"form-data"
	Content string `json:"content"`
}

// HttpResponse は HTTP レスポンスを表す。
type HttpResponse struct {
	StatusCode  int               `json:"statusCode"`
	StatusText  string            `json:"statusText"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	ContentType string            `json:"contentType"`
	Size        int64             `json:"size"`
	TimingMs    int64             `json:"timingMs"`
	Error       string            `json:"error"`
}

// Collection はリクエストコレクションを表す。
type Collection struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Items []TreeItem `json:"items"`
}

// TreeItem はコレクション内のフォルダまたはリクエストアイテムを表す。
type TreeItem struct {
	Type     string       `json:"type"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Children []TreeItem   `json:"children"`
	Request  *HttpRequest `json:"request,omitempty"`
}

// ItemType 定数はツリーアイテムの種別を定義する。
const (
	ItemTypeFolder  = "folder"
	ItemTypeRequest = "request"
)
