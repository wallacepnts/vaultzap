package web

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// Number of --avatar-N-bg/-fg pairs defined in app.css.
const avatarColorCount = 20

func nameHash(name string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return h.Sum32()
}

func avatarClass(name string) string {
	return fmt.Sprintf("avatar-color-%d", nameHash(name)%avatarColorCount)
}

func initials(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	parts := strings.Fields(name)
	first := []rune(parts[0])
	if len(parts) == 1 {
		return strings.ToUpper(string(first[0]))
	}
	last := []rune(parts[len(parts)-1])
	return strings.ToUpper(string(first[0]) + string(last[0]))
}
