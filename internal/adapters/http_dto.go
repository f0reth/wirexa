package adapters

import httpdomain "github.com/f0reth/Wirexa/internal/domain/http"

// KeyValuePair はヘッダーやパラメータのキーバリューペアの RPC 転送型。
type KeyValuePair struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

// RequestBody はリクエストボディの RPC 転送型。
type RequestBody struct {
	Type     string            `json:"type"`
	Contents map[string]string `json:"contents"`
}

// RequestAuth はリクエスト認証情報の RPC 転送型。
type RequestAuth struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

// RequestSettings はリクエスト設定の RPC 転送型。
type RequestSettings struct {
	TimeoutSec         int    `json:"timeoutSec"`
	ProxyMode          string `json:"proxyMode"`
	ProxyURL           string `json:"proxyURL"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	DisableRedirects   bool   `json:"disableRedirects"`
	MaxResponseBodyMB  int    `json:"maxResponseBodyMB"`
}

// HttpRequest は HTTP リクエストの RPC 転送型。
type HttpRequest struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Headers  []KeyValuePair  `json:"headers"`
	Params   []KeyValuePair  `json:"params"`
	Body     RequestBody     `json:"body"`
	Auth     RequestAuth     `json:"auth"`
	Settings RequestSettings `json:"settings"`
	Doc      string          `json:"doc"`
}

// HttpResponse は HTTP レスポンスの RPC 転送型。
type HttpResponse struct {
	StatusCode    int               `json:"statusCode"`
	StatusText    string            `json:"statusText"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	ContentType   string            `json:"contentType"`
	Size          int64             `json:"size"`
	TimingMs      int64             `json:"timingMs"`
	Error         string            `json:"error"`
	BodyTruncated bool              `json:"bodyTruncated"`
	TempFilePath  string            `json:"tempFilePath"` // インフラ詳細: 上限超過時のみ非空
}

// TreeItem はコレクション内アイテムの RPC 転送型。
type TreeItem struct {
	Type     string       `json:"type"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Children []*TreeItem  `json:"children"`
	Request  *HttpRequest `json:"request,omitempty"`
}

// Collection はリクエストコレクションの RPC 転送型。
type Collection struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Items []*TreeItem `json:"items"`
	Order int         `json:"order"`
}

func fromKeyValuePairDTO(kv KeyValuePair) httpdomain.KeyValuePair {
	return httpdomain.KeyValuePair{Key: kv.Key, Value: kv.Value, Enabled: kv.Enabled}
}

func toKeyValuePairDTO(kv httpdomain.KeyValuePair) KeyValuePair {
	return KeyValuePair{Key: kv.Key, Value: kv.Value, Enabled: kv.Enabled}
}

func fromHTTPRequestDTO(req HttpRequest) httpdomain.HttpRequest {
	headers := make([]httpdomain.KeyValuePair, len(req.Headers))
	for i, h := range req.Headers {
		headers[i] = fromKeyValuePairDTO(h)
	}
	params := make([]httpdomain.KeyValuePair, len(req.Params))
	for i, p := range req.Params {
		params[i] = fromKeyValuePairDTO(p)
	}
	return httpdomain.HttpRequest{
		ID:      req.ID,
		Name:    req.Name,
		Method:  req.Method,
		URL:     req.URL,
		Headers: headers,
		Params:  params,
		Body:    httpdomain.RequestBody{Type: req.Body.Type, Contents: req.Body.Contents},
		Auth: httpdomain.RequestAuth{
			Type:     req.Auth.Type,
			Username: req.Auth.Username,
			Password: req.Auth.Password,
			Token:    req.Auth.Token,
		},
		Settings: httpdomain.RequestSettings{
			TimeoutSec:         req.Settings.TimeoutSec,
			ProxyMode:          req.Settings.ProxyMode,
			ProxyURL:           req.Settings.ProxyURL,
			InsecureSkipVerify: req.Settings.InsecureSkipVerify,
			DisableRedirects:   req.Settings.DisableRedirects,
			MaxResponseBodyMB:  req.Settings.MaxResponseBodyMB,
		},
		Doc: req.Doc,
	}
}

func toHTTPRequestDTO(req httpdomain.HttpRequest) HttpRequest {
	headers := make([]KeyValuePair, len(req.Headers))
	for i, h := range req.Headers {
		headers[i] = toKeyValuePairDTO(h)
	}
	params := make([]KeyValuePair, len(req.Params))
	for i, p := range req.Params {
		params[i] = toKeyValuePairDTO(p)
	}
	return HttpRequest{
		ID:      req.ID,
		Name:    req.Name,
		Method:  req.Method,
		URL:     req.URL,
		Headers: headers,
		Params:  params,
		Body:    RequestBody{Type: req.Body.Type, Contents: req.Body.Contents},
		Auth: RequestAuth{
			Type:     req.Auth.Type,
			Username: req.Auth.Username,
			Password: req.Auth.Password,
			Token:    req.Auth.Token,
		},
		Settings: RequestSettings{
			TimeoutSec:         req.Settings.TimeoutSec,
			ProxyMode:          req.Settings.ProxyMode,
			ProxyURL:           req.Settings.ProxyURL,
			InsecureSkipVerify: req.Settings.InsecureSkipVerify,
			DisableRedirects:   req.Settings.DisableRedirects,
			MaxResponseBodyMB:  req.Settings.MaxResponseBodyMB,
		},
		Doc: req.Doc,
	}
}

func toHTTPResponseDTO(res httpdomain.HttpResponse, tempFilePath string) HttpResponse {
	return HttpResponse{
		StatusCode:    res.StatusCode,
		StatusText:    res.StatusText,
		Headers:       res.Headers,
		Body:          res.Body,
		ContentType:   res.ContentType,
		Size:          res.Size,
		TimingMs:      res.TimingMs,
		Error:         res.Error,
		BodyTruncated: res.BodyTruncated,
		TempFilePath:  tempFilePath,
	}
}

func toTreeItemDTO(item *httpdomain.TreeItem) *TreeItem {
	if item == nil {
		return nil
	}
	children := make([]*TreeItem, len(item.Children))
	for i, c := range item.Children {
		children[i] = toTreeItemDTO(c)
	}
	var reqDTO *HttpRequest
	if item.Request != nil {
		dto := toHTTPRequestDTO(*item.Request)
		reqDTO = &dto
	}
	return &TreeItem{
		Type:     item.Type,
		ID:       item.ID,
		Name:     item.Name,
		Children: children,
		Request:  reqDTO,
	}
}

func toCollectionDTO(col httpdomain.Collection) Collection {
	items := make([]*TreeItem, len(col.Items))
	for i, item := range col.Items {
		items[i] = toTreeItemDTO(item)
	}
	return Collection{ID: col.ID, Name: col.Name, Items: items, Order: col.Order}
}

// SidebarEntryDTO はサイドバーエントリの RPC 転送型。
type SidebarEntryDTO struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func toSidebarEntryDTO(e httpdomain.SidebarEntry) SidebarEntryDTO {
	return SidebarEntryDTO{Kind: e.Kind, ID: e.ID}
}
