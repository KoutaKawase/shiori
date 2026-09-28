package app

import (
	"fmt"
	"strings"

	"shiori/internal/domain"
)

// ToFTSMatchQuery はユーザ入力を安全なFTS5 MATCH式に変換する。
//SQL文字列結合はせず、MATCH全体を1つの束縛パラメータとして渡す。
//各トークンをダブルクォートで囲みANDで結合する: `docker compose` -> `"docker" AND "compose"`。
func ToFTSMatchQuery(raw string) (string, error) {
	s, err := domain.ValidateFTSQuery(raw)
	if err != nil {
		return "", err
	}
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return "", fmt.Errorf("query must not be empty")
	}
	quoted := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		// FTS5引用符内では "" でエスケープする。
		esc := strings.ReplaceAll(tok, `"`, `""`)
		quoted = append(quoted, `"`+esc+`"`)
	}
	return strings.Join(quoted, " AND "), nil
}
