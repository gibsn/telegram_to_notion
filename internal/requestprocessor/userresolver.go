package requestprocessor

import (
	"fmt"
	"strings"
)

type UserResolver struct {
	tgToNotion map[string]string
	notionToTg map[string]string
}

func NewUserResolver() *UserResolver {
	r := &UserResolver{}

	r.tgToNotion = map[string]string{
		"@alexander_zh": "9e8f4963-fd1c-4bb5-bdd2-7f29a9a8698a",
		"@vomadan":      "0724b18e-320d-4fce-87f6-95d69b51c2c0",
		"@fenyakolles":  "78694531-146f-4abd-b29b-093278cab708",
		"@nikitacmc":    "e6f7887a-7123-4a83-a5da-ded24467d5e2",
		"@homesick94":   "3c02801c-1a5a-428f-b217-6d53032a21c9",
		"@gibsn":        "7439e2ca-75f8-4024-b170-620ef7ed08b1",
		"@bond_lullaby": "aea80e9c-7a69-4180-8a38-6d274af25f4c",
	}

	r.notionToTg = map[string]string{
		"9e8f4963-fd1c-4bb5-bdd2-7f29a9a8698a": "@alexander_zh",
		"0724b18e-320d-4fce-87f6-95d69b51c2c0": "@vomadan",
		"78694531-146f-4abd-b29b-093278cab708": "@fenyakolles",
		"e6f7887a-7123-4a83-a5da-ded24467d5e2": "@nikitacmc",
		"3c02801c-1a5a-428f-b217-6d53032a21c9": "@homesick94",
		"7439e2ca-75f8-4024-b170-620ef7ed08b1": "@gibsn",
		"aea80e9c-7a69-4180-8a38-6d274af25f4c": "@bond_lullaby",
	}

	return r
}

func (r *UserResolver) TgToNotion(tgName string) string {
	return r.tgToNotion[strings.ToLower(strings.TrimSpace(tgName))]
}

func (r *UserResolver) NotionToTg(notionID string) string {
	return r.notionToTg[strings.ToLower(strings.TrimSpace(notionID))]
}

func (r *UserResolver) ResolveArr(tgNames []string) ([]string, error) {
	resolved := make([]string, 0, len(tgNames))

	for _, tgName := range tgNames {
		resolvedName := r.TgToNotion(tgName)
		if resolvedName == "" {
			return nil, fmt.Errorf("login unknown: %s", tgName)
		}

		resolved = append(resolved, resolvedName)
	}

	return resolved, nil
}
