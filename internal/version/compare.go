package version

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Match evaluates whether actual satisfies the constraint expression.
// Supported operators: >=, <=, >, <, ==, =, !=.
// If no operator prefix is given, ">=" is assumed (i.e. "at least this version is released").
func Match(actual, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	actual = strings.TrimSpace(actual)
	if constraint == "" || constraint == "*" {
		return actual != "", nil
	}

	op := ">="
	target := constraint

	for _, candidate := range []string{">=", "<=", "!=", "==", ">", "<", "="} {
		if strings.HasPrefix(constraint, candidate) {
			op = candidate
			target = strings.TrimSpace(strings.TrimPrefix(constraint, candidate))
			break
		}
	}

	if target == "" {
		return false, fmt.Errorf("empty version target in constraint %q", constraint)
	}

	cmp := Compare(actual, target)
	switch op {
	case ">=":
		return cmp >= 0, nil
	case "<=":
		return cmp <= 0, nil
	case ">":
		return cmp > 0, nil
	case "<":
		return cmp < 0, nil
	case "==", "=":
		return cmp == 0, nil
	case "!=":
		return cmp != 0, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", op)
	}
}

// Compare compares two version strings (handling leading 'v', multi-segment numbers,
// snap version strings like "5.21.2-20240812" or "16.4", and pre-release suffixes).
// Returns -1 if a < b, 0 if a == b, +1 if a > b.
func Compare(a, b string) int {
	a = normalize(a)
	b = normalize(b)
	if a == b {
		return 0
	}

	aMain, aPre := splitPreRelease(a)
	bMain, bPre := splitPreRelease(b)

	aTokens := tokenize(aMain)
	bTokens := tokenize(bMain)

	maxLen := len(aTokens)
	if len(bTokens) > maxLen {
		maxLen = len(bTokens)
	}

	for i := 0; i < maxLen; i++ {
		var tokA, tokB string
		if i < len(aTokens) {
			tokA = aTokens[i]
		} else {
			tokA = "0"
		}
		if i < len(bTokens) {
			tokB = bTokens[i]
		} else {
			tokB = "0"
		}

		if cmp := compareToken(tokA, tokB); cmp != 0 {
			return cmp
		}
	}

	// In semver, having no pre-release is greater than having a pre-release (e.g. 1.0.0 > 1.0.0-alpha).
	if aPre == "" && bPre == "" {
		return 0
	}
	if aPre == "" && bPre != "" {
		return 1
	}
	if aPre != "" && bPre == "" {
		return -1
	}

	aPreTokens := tokenize(aPre)
	bPreTokens := tokenize(bPre)
	maxPre := len(aPreTokens)
	if len(bPreTokens) > maxPre {
		maxPre = len(bPreTokens)
	}
	for i := 0; i < maxPre; i++ {
		if i >= len(aPreTokens) {
			return -1
		}
		if i >= len(bPreTokens) {
			return 1
		}
		if cmp := compareToken(aPreTokens[i], bPreTokens[i]); cmp != 0 {
			return cmp
		}
	}
	return 0
}

func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	// Strip build metadata (+...)
	if idx := strings.IndexByte(v, '+'); idx != -1 {
		v = v[:idx]
	}
	return v
}

func splitPreRelease(v string) (string, string) {
	if idx := strings.IndexAny(v, "-~"); idx != -1 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

func tokenize(s string) []string {
	var tokens []string
	var cur strings.Builder
	var lastIsDigit bool

	for i, r := range s {
		if r == '.' || r == '_' || r == '-' {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			continue
		}
		isDigit := unicode.IsDigit(r)
		if i > 0 && cur.Len() > 0 && isDigit != lastIsDigit {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
		cur.WriteRune(r)
		lastIsDigit = isDigit
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func compareToken(a, b string) int {
	na, errA := strconv.ParseInt(a, 10, 64)
	nb, errB := strconv.ParseInt(b, 10, 64)
	if errA == nil && errB == nil {
		if na < nb {
			return -1
		}
		if na > nb {
			return 1
		}
		return 0
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
