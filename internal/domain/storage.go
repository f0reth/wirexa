package domain

// Quarantiner は壊れた保存ファイルを上書きされない場所へ退避する出力ポート。
// 一覧全体を 1 ファイルで読み書きするリポジトリが埋め込む。
type Quarantiner interface {
	// Quarantine は壊れたファイルを退避し、退避先のパスを返す。
	Quarantine() (string, error)
}
