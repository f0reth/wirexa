package httpdomain

import "errors"

// RPC 境界へ返す分類済みエラー。token・execution ID・実パス・一時ファイルパスを
// メッセージに含めないため、固定文言の sentinel として定義する。
// 呼び出し側は errors.Is で判定し、OS エラーを %w でこれらに連結しないこと。
var (
	// ErrResponseUnavailable は execution ID が未知・期限切れ・保存済みであることを示す。
	ErrResponseUnavailable = errors.New("response body unavailable")
	// ErrResponseBusy は保存・破棄・同じ ID の送信が競合したことを示す。
	ErrResponseBusy = errors.New("response body busy")
	// ErrResponseStorageLimit は一時ファイルの件数・総容量・同時 spill の上限到達を示す。
	ErrResponseStorageLimit = errors.New("response storage limit exceeded")
	// ErrSaveResponseFailed は保存先への copy や元ファイル削除の失敗を示す。
	ErrSaveResponseFailed = errors.New("failed to save response")
	// ErrDiscardResponseFailed は一時ファイルの削除に失敗したことを示す。
	ErrDiscardResponseFailed = errors.New("failed to discard response")
	// ErrExecutionInProgress は同じ execution ID のリクエストが実行中であることを示す。
	ErrExecutionInProgress = errors.New("a request with the same execution ID is already running")

	// ErrFileAccessDenied は file token が空・未知・失効済み、または再選択待ちであることを示す。
	ErrFileAccessDenied = errors.New("file access denied: select the file again")
	// ErrSelectedFileUnavailable は選択後の削除・権限変更などでファイルを読めないことを示す。
	ErrSelectedFileUnavailable = errors.New("selected file unavailable")
	// ErrSelectedFileChanged は選択したファイルが送信中に書き換えられたことを示す。
	ErrSelectedFileChanged = errors.New("selected file changed while sending")
	// ErrSelectedFileInUse は他のプロセスが書き込み用に開いていて、選択したファイルを開けないことを示す。
	ErrSelectedFileInUse = errors.New("selected file is in use by another process")
	// ErrFileSelectionLimit は選択済みファイルの登録数が上限に達したことを示す。
	ErrFileSelectionLimit = errors.New("file selection limit reached")
)
