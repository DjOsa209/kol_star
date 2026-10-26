package main

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

// Links are stored as references only. They are never fetched or sent to translation.
func ipAssetLinksIssue(profile map[string]any) string {
	value, present := profile["assetLinks"]
	if !present {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return "素材链接应为文本，每行填写一个URL"
	}
	count := 0
	for _, line := range strings.Split(text, "\n") {
		link := strings.TrimSpace(line)
		if link == "" {
			continue
		}
		count++
		if count > 20 {
			return "最多填写20个素材链接"
		}
		parsed, err := url.Parse(link)
		invalid := len(utf16.Encode([]rune(link))) > 2048 || strings.Contains(link, `\`) || strings.IndexFunc(link, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0
		if err != nil || invalid || parsed.Hostname() == "" || parsed.User != nil || (strings.ToLower(parsed.Scheme) != "http" && strings.ToLower(parsed.Scheme) != "https") {
			return "素材链接请输入有效的HTTP或HTTPS地址，每行一个，单条不超过2048个字符"
		}
		if port := parsed.Port(); port != "" {
			if number, err := strconv.Atoi(port); err != nil || number > 65535 {
				return "素材链接请输入有效的HTTP或HTTPS地址，每行一个，单条不超过2048个字符"
			}
		}
	}
	return ""
}
