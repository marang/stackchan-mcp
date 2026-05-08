package app

import "strings"

func splitArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
func numberArg(args map[string]any, name string, fallback int) int {
	value, ok := args[name]
	if !ok {
		return fallback
	}
	number, ok := value.(float64)
	if !ok {
		return fallback
	}
	return int(number)
}

func boolArg(args map[string]any, name string, fallback bool) bool {
	value, ok := args[name]
	if !ok {
		return fallback
	}
	boolean, ok := value.(bool)
	if !ok {
		return fallback
	}
	return boolean
}
