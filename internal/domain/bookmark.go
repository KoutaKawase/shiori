package domain

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	MaxURLLength     = 2048
	MaxTitleLength   = 500
	MaxCommentLength = 10000
	MaxTagLength     = 64
	MaxFTSQueryLen   = 500
)

// ValidateURL はブックマークURLを検証し、正規化された文字列表現を返す。
// http/httpsのみ許可する。
func ValidateURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("url must not be empty")
	}
	if len(s) > MaxURLLength {
		return "", fmt.Errorf("url too long: max %d bytes", MaxURLLength)
	}
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("url scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("url must have a host")
	}
	return s, nil
}

// NormalizeTag はタグを正規化する。前後空白除去+小文字化。
// フラットタグのみ。階層記号は拒否する。
func NormalizeTag(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", fmt.Errorf("tag must not be empty")
	}
	if len(s) > MaxTagLength {
		return "", fmt.Errorf("tag too long: max %d bytes", MaxTagLength)
	}
	if utf8.RuneCountInString(s) < 1 {
		return "", fmt.Errorf("tag must not be empty")
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return "", fmt.Errorf("invalid tag %q: use [a-z0-9-_]", raw)
		}
	}
	return s, nil
}

// NormalizeTags は重複除去つきでタグ列を正規化する。ソートはしない。
func NormalizeTags(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		n, err := NormalizeTag(t)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out, nil
}

// ValidateTitle はタイトルを検証する。空は許可する。
func ValidateTitle(s string) (string, error) {
	if len(s) > MaxTitleLength {
		return "", fmt.Errorf("title too long: max %d bytes", MaxTitleLength)
	}
	return s, nil
}

// ValidateComment はコメントを検証する。空は許可する。
func ValidateComment(s string) (string, error) {
	if len(s) > MaxCommentLength {
		return "", fmt.Errorf("comment too long: max %d bytes", MaxCommentLength)
	}
	return s, nil
}

// ValidateFTSQuery は全文検索クエリの事前検証を行う。
// FTS5 MATCH組み立て前の長さ・空チェックのみ。構文の無害化はapp層で行う。
func ValidateFTSQuery(q string) (string, error) {
	s := strings.TrimSpace(q)
	if s == "" {
		return "", fmt.Errorf("query must not be empty")
	}
	if len(s) > MaxFTSQueryLen {
		return "", fmt.Errorf("query too long: max %d bytes", MaxFTSQueryLen)
	}
	return s, nil
}

// ValidateLimitOffset は一覧系のページング値を検証・補正する。
func ValidateLimitOffset(limit, offset int) (int, int, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		return 0, 0, fmt.Errorf("limit too large: max %d", MaxLimit)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("offset must be >= 0")
	}
	return limit, offset, nil
}
