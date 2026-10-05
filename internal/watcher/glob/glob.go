package glob

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

func Match(pattern, path string) bool {
	pattern = strings.ReplaceAll(pattern, `\`, "/")
	path = strings.ReplaceAll(path, `\`, "/")

	expression, err := compile(pattern)
	return err == nil && expression.MatchString(path)
}

func compile(pattern string) (*regexp.Regexp, error) {
	var expression strings.Builder
	expression.WriteString("(?s)^")

	for index := 0; index < len(pattern); {
		switch pattern[index] {
		case '*':
			if index+1 < len(pattern) && pattern[index+1] == '*' {
				if index+2 < len(pattern) && pattern[index+2] == '/' {
					expression.WriteString("(.*/)?")
					index += 3
				} else {
					expression.WriteString(".*")
					index += 2
				}
			} else {
				expression.WriteString("[^/]*")
				index++
			}
		case '?':
			expression.WriteString("[^/]")
			index++
		case '[':
			end := strings.IndexByte(pattern[index+1:], ']')
			if end < 0 {
				return nil, errors.New("invalid character class")
			}
			end += index + 1
			class := pattern[index+1 : end]
			if !appendClass(&expression, class) {
				return nil, errors.New("invalid character class")
			}
			index = end + 1
		default:
			runeValue, size := utf8.DecodeRuneInString(pattern[index:])
			expression.WriteString(regexp.QuoteMeta(string(runeValue)))
			index += size
		}
	}

	expression.WriteByte('$')
	return regexp.Compile(expression.String())
}

func appendClass(expression *strings.Builder, class string) bool {
	if class == "" {
		return false
	}

	negated := class[0] == '!' || class[0] == '^'
	if negated {
		class = class[1:]
		if class == "" {
			return false
		}
		expression.WriteString("[^/")
		expression.WriteString(class)
		expression.WriteByte(']')
		return true
	}

	class = strings.ReplaceAll(class, "/", "")
	if class == "" {
		return false
	}
	expression.WriteByte('[')
	expression.WriteString(class)
	expression.WriteByte(']')
	return true
}
