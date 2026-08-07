package web

import (
	"time"

	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

type ChatView struct {
	store.Chat
	PreviewTime string
	// The user's lists, each marked with whether this chat is in it.
	Lists []TaggedList
}

type TaggedList struct {
	ID   int64
	Name string
	In   bool
}

func buildChatViews(chats []store.Chat, lists []store.List, assoc map[int64]map[int64]bool, now time.Time, lang render.Locale) []ChatView {
	today := now.Format("2006-01-02")

	views := make([]ChatView, len(chats))
	for i, c := range chats {
		v := ChatView{Chat: c, PreviewTime: timeOrShortDate(c.LastMessageAt, today, lang)}
		if len(lists) > 0 {
			v.Lists = make([]TaggedList, len(lists))
			for j, l := range lists {
				v.Lists[j] = TaggedList{ID: l.ID, Name: l.Name, In: assoc[c.ID][l.ID]}
			}
		}
		views[i] = v
	}
	return views
}

func timeOrShortDate(sentAt, today string, lang render.Locale) string {
	if len(sentAt) < 16 {
		return ""
	}
	date := sentAt[:10]
	if date == today {
		if t, err := time.Parse("2006-01-02 15:04:05", sentAt); err == nil {
			return render.ClockTime(t, lang)
		}
		return sentAt[11:16]
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return render.ShortDate(t, lang)
}
