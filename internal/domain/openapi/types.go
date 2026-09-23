// Package openapidomain は OpenAPI ファイル操作のドメイン型とポートを定義する。
package openapidomain

// OpenAPIRecent は最近開いた OpenAPI ファイルの参照。
// path を一意キーとして扱う (id は持たない)。
type OpenAPIRecent struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	LastOpenedAt string `json:"lastOpenedAt"`
	Order        int    `json:"order"`
}
