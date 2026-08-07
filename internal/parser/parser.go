// Package parser turns the raw text of a WhatsApp export (Android or iOS) into a
// structured list of messages.
//
// It never panics and never discards the whole file over one bad line: the worst case
// is a kind="system" message holding the raw text, plus a warning in the result.
package parser

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

type Message struct {
	SentAt         string  `json:"sent_at"`
	Seq            int     `json:"seq"`
	Sender         *string `json:"sender"`
	Body           string  `json:"body"`
	Kind           string  `json:"kind"`
	AttachmentName string  `json:"attachment_name,omitempty"`
	MediaKind      string  `json:"media_kind,omitempty"`
}

type Result struct {
	Source    string    `json:"source"` // android | ios
	DateOrder string    `json:"date_order"`
	IsGroup   bool      `json:"is_group"`
	Messages  []Message `json:"messages"`
	Warnings  []string  `json:"warnings"`
	// Checks is triage over the file's own shape (§11.40), not part of what was parsed —
	// hence out of the golden files, which exist to pin parsing.
	Checks []Check `json:"-"`
}

type Options struct {
	// Used when the DD/MM order is still ambiguous after inference. Empty means OrderDMY.
	DefaultDateOrder DateOrder

	// ChatName is the name derived from the file, used to recognize group notices signed
	// with the group's name in place of the sender. Empty disables that reclassification.
	ChatName string
}

// rawMessage is the first pass's output: header decoded and continuations concatenated,
// but with no resolved date order yet.
type rawMessage struct {
	seq           int
	date          dateComponents
	time          parsedTime
	noDate        bool // header structurally recognized but with an invalid date/time
	rest          string
	continuations []string
}

// Parse detects the dialect, resolves the ambiguous date order and classifies each line.
func Parse(r io.Reader, opts Options) (Result, error) {
	defaultOrder := opts.DefaultDateOrder
	if defaultOrder == "" {
		defaultOrder = OrderDMY
	}

	content, err := io.ReadAll(r)
	if err != nil {
		return Result{}, fmt.Errorf("read content: %w", err)
	}

	text := normalize(content)
	lines := strings.Split(text, "\n")
	// Split of text ending in "\n" produces a spurious trailing empty line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	dialect, err := detectDialect(lines)
	if err != nil {
		return Result{}, err
	}

	raw, warnings := groupMessages(dialect, lines)
	order := inferOrder(raw, defaultOrder)

	messages := make([]Message, 0, len(raw))
	senders := map[string]bool{}
	isGroup := false

	for _, b := range raw {
		msg := finalizeMessage(b, order)
		messages = append(messages, msg)
		if msg.Sender != nil {
			senders[*msg.Sender] = true
		}
		if msg.Kind == "system" && matchesAny(groupSystemPatterns, msg.Body) {
			isGroup = true
		}
	}
	if len(senders) > 2 {
		isGroup = true
	}

	if chatNameIsGroup(messages, opts.ChatName) {
		isGroup = true
		for i := range messages {
			reclassifyGroupNotice(&messages[i], opts.ChatName)
		}
	}

	return Result{
		Source:    dialect.String(),
		DateOrder: string(order),
		IsGroup:   isGroup,
		Messages:  messages,
		Warnings:  warnings,
		Checks:    coherenceChecks(messages, lines, marksByLine(content)),
	}, nil
}

func groupMessages(dialect Dialect, lines []string) ([]rawMessage, []string) {
	var raw []rawMessage
	var warnings []string
	var current *rawMessage

	closeCurrent := func() {
		if current != nil {
			raw = append(raw, *current)
			current = nil
		}
	}

	for i, line := range lines {
		header, state := matchHeader(dialect, line)
		switch state {
		case headerValid:
			closeCurrent()
			current = &rawMessage{seq: len(raw) + 1, date: header.date, time: header.time, rest: header.rest}
		case headerInvalid:
			closeCurrent()
			warnings = append(warnings, fmt.Sprintf("line %d: invalid date/time, treated as a system message", i+1))
			current = &rawMessage{seq: len(raw) + 1, noDate: true, rest: header.rest}
		default: // headerNone
			if current != nil {
				current.continuations = append(current.continuations, line)
			} else if strings.TrimSpace(line) != "" {
				warnings = append(warnings, fmt.Sprintf("line %d: outside any message, ignored", i+1))
			}
		}
	}
	closeCurrent()

	return raw, warnings
}

// Applies under a corrupt header too, where dropping the continuations would silently lose
// every line but the first while the warning only mentions that one.
func (b rawMessage) withContinuations(firstLine string) string {
	if len(b.continuations) == 0 {
		return firstLine
	}
	return firstLine + "\n" + strings.Join(b.continuations, "\n")
}

func finalizeMessage(b rawMessage, order DateOrder) Message {
	if b.noDate {
		return Message{Seq: b.seq, Body: b.withContinuations(b.rest), Kind: "system"}
	}

	sentAt := formatDate(b.date, b.time, order)

	var sender *string
	firstLineBody := b.rest
	if m := bodyRe.FindStringSubmatch(b.rest); m != nil {
		s := m[1]
		sender = &s
		firstLineBody = m[2]
	}

	body := b.withContinuations(firstLineBody)

	kind := "text"
	attachmentName, mediaKind := "", ""

	switch {
	case sender == nil:
		kind = "system"
	case matchesAny(deletedPatterns, firstLineBody):
		kind = "deleted"
	default:
		if name, ok := extractAttachment(firstLineBody); ok {
			attachmentName = name
			mediaKind = classifyMedia(name)
			if mediaKind == "contact" {
				kind = "contact"
			} else {
				kind = "media"
			}
		} else if matchesAny(omittedMediaPatterns, firstLineBody) {
			kind = "media_omitted"
		} else if matchesAny(callPatterns, firstLineBody) {
			kind = "call"
		} else if matchesAny(locationPatterns, firstLineBody) {
			kind = "location"
		} else if matchesAny(generalSystemPatterns, firstLineBody) {
			kind = "system"
			sender = nil
		}
	}

	return Message{
		SentAt:         sentAt,
		Seq:            b.seq,
		Sender:         sender,
		Body:           body,
		Kind:           kind,
		AttachmentName: attachmentName,
		MediaKind:      mediaKind,
	}
}

// Strips WhatsApp's known prefixes; without one, the file name minus its extension.
func ChatNameFromFile(fileName string) string {
	base := filepath.Base(fileName)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	for _, prefix := range fileNamePrefixes {
		if strings.HasPrefix(base, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(base, prefix))
		}
	}
	return base
}

// Digits, spaces, and the punctuation WhatsApp uses in exported numbers.
var phoneCharsRe = regexp.MustCompile(`^[0-9\s+().-]+$`)

// WhatsApp names the file after the number when the contact isn't saved. At least 7
// digits, so a short numeric nickname ("007") isn't misdetected.
func LooksLikePhoneNumber(name string) bool {
	name = normalizeName(name)
	if !phoneCharsRe.MatchString(name) {
		return false
	}
	digits := 0
	for _, r := range name {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 7
}

// Whether messages signed with the chat's name are notices rather than a person talking:
// every one of them matches a group-notice pattern, and another sender exists.
func chatNameIsGroup(messages []Message, chatName string) bool {
	target := strings.TrimSpace(chatName)
	if target == "" {
		return false
	}

	notices, otherSenders := 0, 0
	for _, m := range messages {
		if m.Sender == nil {
			continue
		}
		if !SameName(*m.Sender, target) {
			otherSenders++
			continue
		}
		if m.Kind != "text" || !matchesAny(strictGroupNoticePatterns, strings.TrimSpace(m.Body)) {
			return false
		}
		notices++
	}
	return notices > 0 && otherSenders > 0
}

// Clears the sender of a system notice signed with the group's name.
func reclassifyGroupNotice(msg *Message, chatName string) {
	if msg.Sender == nil || msg.Kind != "text" {
		return
	}
	if !SameName(*msg.Sender, chatName) {
		return
	}
	msg.Sender = nil
	msg.Kind = "system"
}
