package web

import (
	"fmt"
	"time"

	"github.com/wallacepnts/vaultzap/internal/locale"
	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

type MessageView struct {
	store.Message
	ShortTime      string
	FirstInBlock   bool // controls the bubble's tail
	ShowSenderName bool // name shown above the bubble
	Mine           bool // bubble on the right
	DateDivider    string
	SenderClass    string // avatar-color-N: the name takes the person's avatar hue
	SenderName     string // .Sender deref, empty for a system message
	SenderRenamed  bool   // display name differs from the export's, so both can be shown
	ChatID         int64
	ChatAvatarPath string
	// AvatarURL is what the voice player shows: the participant's own photo when the user
	// set one, the chat's otherwise. Empty renders initials.
	AvatarURL   string
	Highlighted bool // set when opened from a search result (?around=)
	// Consecutive stickers from the same sender share one header and sit side by side,
	// like the official app: these mark where that strip opens and closes.
	StickerRunStart bool
	StickerRunEnd   bool
	IsGroup         bool // a sticker only gets the bubble-with-header treatment in a group
}

type conversationContext struct {
	Owner      string
	IsGroup    bool
	ChatID     int64
	AvatarPath string
	// Sender as the export has it -> the name the user chose. Empty shows the original.
	Nicknames map[string]string
	// Sender as the export has it -> the photo URL the user attached, if any.
	Avatars map[string]string
	// OwnerName is what the user called themselves in their profile; it replaces the
	// export's sender for whoever is the owner here.
	OwnerName string
	Locale    render.Locale
}

func (c conversationContext) displayName(sender string) string {
	if nickname, ok := c.Nicknames[sender]; ok && nickname != "" {
		return nickname
	}
	if sender == c.Owner && c.OwnerName != "" {
		return c.OwnerName
	}
	return sender
}

// Groups consecutive messages from the same sender and computes the date dividers, which
// compare against today's local date. Messages arrive chronologically.
func buildMessageViews(messages []store.Message, ctx conversationContext, now time.Time, highlightID int64) []MessageView {
	views := make([]MessageView, len(messages))
	previousDate := ""
	var previousSender *string

	for i, m := range messages {
		v := MessageView{Message: m}
		v.Highlighted = highlightID != 0 && m.ID == highlightID
		v.ChatID = ctx.ChatID
		v.IsGroup = ctx.IsGroup
		if t, err := time.Parse("2006-01-02 15:04:05", m.SentAt); err == nil {
			v.ShortTime = render.ClockTime(t, ctx.Locale)
		}

		date := m.SentAt
		if len(date) >= 10 {
			date = date[:10]
		}
		if date != previousDate {
			v.DateDivider = dateLabel(date, now, ctx.Locale)
			previousDate = date
		}

		newBlock := v.DateDivider != "" || !sameSender(previousSender, m.Sender)
		v.FirstInBlock = newBlock

		if m.Sender != nil {
			v.Mine = ctx.Owner != "" && *m.Sender == ctx.Owner
			v.SenderName = ctx.displayName(*m.Sender)
			v.SenderRenamed = v.SenderName != *m.Sender
			v.SenderClass = avatarClass(v.SenderName)
			v.ShowSenderName = ctx.IsGroup && newBlock && !v.Mine
			if !v.Mine {
				v.ChatAvatarPath = ctx.AvatarPath
				v.AvatarURL = ctx.Avatars[*m.Sender]
				if v.AvatarURL == "" && ctx.AvatarPath != "" && !ctx.IsGroup {
					v.AvatarURL = fmt.Sprintf("/chats/%d/avatar", ctx.ChatID)
				}
			}
		}

		previousSender = m.Sender
		views[i] = v
	}

	markStickerRuns(views)
	return views
}

func markStickerRuns(views []MessageView) {
	isSticker := func(i int) bool {
		return i >= 0 && i < len(views) && views[i].AttachmentMediaKind == "sticker"
	}
	for i := range views {
		if !isSticker(i) {
			continue
		}
		views[i].StickerRunStart = !isSticker(i-1) || views[i].FirstInBlock
		views[i].StickerRunEnd = !isSticker(i+1) || (i+1 < len(views) && views[i+1].FirstInBlock)
	}
}

func dateTimeDisplayFunc(lang render.Locale) func(string) string {
	return func(sentAt string) string {
		t, err := time.Parse("2006-01-02 15:04:05", sentAt)
		if err != nil {
			return ""
		}
		return render.ShortDate(t, lang) + " " + locale.T(lang, "date.connector") + " " + render.ClockTime(t, lang)
	}
}

func sameSender(a, b *string) bool {
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func dateLabel(date string, now time.Time, lang render.Locale) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	today, _ := time.Parse("2006-01-02", now.Format("2006-01-02"))
	days := int(today.Sub(t).Hours() / 24)

	switch {
	case days == 0:
		return render.TodayLabel(lang)
	case days == 1:
		return render.YesterdayLabel(lang)
	case days > 1 && days < 7:
		return render.WeekdayName(t.Weekday(), lang)
	default:
		return render.LongDate(date, lang)
	}
}
