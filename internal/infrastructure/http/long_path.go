package httpinfra

import "strings"

// longPath は MAX_PATH を超える Windows の絶対パスに \\?\ を付ける。os.Open は内部で同じ変換をするが、
// openSelectedFile (Windows) は CreateFile を直接呼ぶのでここで行う。
// 文字列の処理だけで OS に依存しないので、Windows 以外の CI でもテストできるようビルドタグの無いファイルに置く。
func longPath(path string) string {
	const maxDirPath = 248 // CreateDirectory の制限。os パッケージと同じ閾値を使う
	if len(path) < maxDirPath || !isWindowsAbs(path) || strings.HasPrefix(path, `\\?\`) {
		return path
	}
	if rest, ok := strings.CutPrefix(path, `\\`); ok {
		return `\\?\UNC\` + rest
	}
	return `\\?\` + path
}

// isWindowsAbs は path が Windows の絶対パス (ドライブ文字と区切り文字で始まるか、UNC) かを返す。
// filepath.IsAbs は実行中の OS の規則で判定するので、OS に依存しないようここで判定する。
func isWindowsAbs(path string) bool {
	isSep := func(c byte) bool { return c == '\\' || c == '/' }
	if len(path) >= 2 && isSep(path[0]) && isSep(path[1]) {
		return true
	}
	if len(path) < 3 || path[1] != ':' || !isSep(path[2]) {
		return false
	}
	c := path[0]
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
