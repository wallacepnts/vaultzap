package web

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/wallacepnts/vaultzap/internal/export"
	"github.com/wallacepnts/vaultzap/internal/ingest"
	"github.com/wallacepnts/vaultzap/internal/locale"
	"github.com/wallacepnts/vaultzap/internal/parser"
	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	h.renderFullPage(w, r, "conversation-empty.html", nil)
}

func (h *Handler) chatList(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildChatList(r.Context(), filterFromURL(r), requestLocale(r))
	if err != nil {
		h.internalError(w, "list chats", err)
		return
	}
	h.render(w, r, "chat-list.html", data)
}

func requestLocale(r *http.Request) render.Locale {
	if c, err := r.Cookie("locale"); err == nil {
		return render.Locale(c.Value).Normalize()
	}
	return browserLocale(r.Header.Get("Accept-Language"))
}

// browserLocaleFullTags is checked before browserLocalePrefixes: it is the only
// way to tell an explicit "pt-PT" tag from bare "pt", which falls back to pt-BR.
var browserLocaleFullTags = map[string]render.Locale{
	"pt-pt": render.LocalePT,
}

var browserLocalePrefixes = map[string]render.Locale{
	"pt": render.LocalePTBR,
	"en": render.LocaleEN,
	"es": render.LocaleES,
	"it": render.LocaleIT,
	"fr": render.LocaleFR,
	"de": render.LocaleDE,
	"nl": render.LocaleNL,
}

func browserLocale(header string) render.Locale {
	for _, part := range strings.Split(header, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag = strings.ToLower(tag)
		if locale, ok := browserLocaleFullTags[tag]; ok {
			return locale
		}
		prefix, _, _ := strings.Cut(tag, "-")
		if locale, ok := browserLocalePrefixes[prefix]; ok {
			return locale
		}
	}
	return render.LocalePTBR
}

func filterFromURL(r *http.Request) store.ChatFilter {
	q := r.URL.Query()
	filter := store.ChatFilter{
		Search:   q.Get("q"),
		Archived: q.Get("arquivadas") == "1",
	}
	switch q.Get("filtro") {
	case "favoritas":
		filter.Favorites = true
	case "grupos":
		filter.Groups = true
	}
	if id, err := strconv.ParseInt(q.Get("lista"), 10, 64); err == nil && id > 0 {
		filter.ListID = id
	}
	return filter
}

func (h *Handler) buildChatList(ctx context.Context, filter store.ChatFilter, lang render.Locale) (ChatListData, error) {
	chats, err := h.store.ListChats(ctx, filter)
	if err != nil {
		return ChatListData{}, err
	}
	totalArchived, err := h.store.CountArchived(ctx)
	if err != nil {
		return ChatListData{}, err
	}
	totalFavorites, err := h.store.CountByFilter(ctx, store.ChatFilter{Favorites: true})
	if err != nil {
		return ChatListData{}, err
	}
	totalGroups, err := h.store.CountByFilter(ctx, store.ChatFilter{Groups: true})
	if err != nil {
		return ChatListData{}, err
	}
	lists, err := h.store.ListLists(ctx)
	if err != nil {
		return ChatListData{}, err
	}
	assoc, err := h.store.ListAssociations(ctx)
	if err != nil {
		return ChatListData{}, err
	}

	data := ChatListData{
		Chats:          buildChatViews(chats, lists, assoc, time.Now().In(h.tz), lang),
		Locale:         lang,
		Search:         filter.Search,
		Archived:       filter.Archived,
		TotalArchived:  totalArchived,
		TotalFavorites: totalFavorites,
		TotalGroups:    totalGroups,
		Lists:          lists,
		ActiveList:     filter.ListID,
		Photo:          h.hasMyPhoto(),
	}
	switch {
	case filter.Favorites:
		data.ActiveFilter = "favoritas"
	case filter.Groups:
		data.ActiveFilter = "grupos"
	default:
		data.ActiveFilter = "tudo"
	}
	return data, nil
}

func (h *Handler) archiveChat(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	if err := h.store.SetArchived(r.Context(), chat.ID, !chat.Archived); err != nil {
		h.internalError(w, "archive chat", err)
		return
	}
	slog.Info("chat archived", "chat_id", chat.ID, "archived", !chat.Archived)
	h.respondChatList(w, r)
}

func (h *Handler) deleteChat(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}

	mediaPaths, inboxPaths, err := h.store.DeleteChat(r.Context(), chat.ID)
	if err != nil {
		h.internalError(w, "delete chat", err)
		return
	}

	deleted := 0
	for _, relative := range mediaPaths {
		if err := os.Remove(filepath.Join(h.cfg.MediaDir, relative)); err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("delete chat media", "chat_id", chat.ID, "error", err)
			}
			continue
		}
		deleted++
	}

	slog.Info("chat deleted", "chat_id", chat.ID,
		"media_deleted", deleted, "inbox_exports_preserved", len(inboxPaths))

	// From the panel the deleted chat is the open one: the panel closes and the
	// conversation clears, or the screen would keep showing a chat that is gone.
	if r.FormValue("from") == "perfil" {
		list, err := h.buildChatList(r.Context(), filterFromURL(r), requestLocale(r))
		if err != nil {
			h.internalError(w, "list chats", err)
			return
		}
		h.renderWithOOB(w, r, "right-panel-empty", nil, "chat-gone-oob", list)
		return
	}
	h.respondChatList(w, r)
}

func (h *Handler) respondChatList(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildChatList(r.Context(), filterFromURL(r), requestLocale(r))
	if err != nil {
		h.internalError(w, "list chats", err)
		return
	}
	h.render(w, r, "chat-list.html", data)
}

func (h *Handler) conversation(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	messages, hasMore, err := h.store.LastMessagePage(r.Context(), chat.ID, requestedPageSize(r))
	if err != nil {
		h.internalError(w, "list messages", err)
		return
	}
	data, err := h.buildConversationData(r.Context(), chat, messages, hasMore, 0, true, requestLocale(r))
	if err != nil {
		h.internalError(w, "build conversation data", err)
		return
	}
	if !isHTMXRequest(r) {
		h.renderFullPage(w, r, "conversation.html", data)
		return
	}
	h.renderWithOOB(w, r, "conversation.html", data, "right-panel-empty", nil)
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}

	var (
		messages    []store.Message
		hasMore     bool
		highlightID int64
		err         error
	)

	switch {
	case r.URL.Query().Get("around") != "":
		highlightID, err = strconv.ParseInt(r.URL.Query().Get("around"), 10, 64)
		if err != nil {
			http.Error(w, "invalid around", http.StatusBadRequest)
			return
		}
		messages, hasMore, err = h.store.MessagesAround(r.Context(), chat.ID, highlightID)
	case r.URL.Query().Get("before") != "":
		sentAt, seq, id, errCursor := decodeCursor(r.URL.Query().Get("before"))
		if errCursor != nil {
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
		messages, hasMore, err = h.store.MessagesBefore(r.Context(), chat.ID, sentAt, seq, id, requestedPageSize(r))
	default:
		messages, hasMore, err = h.store.LastMessagePage(r.Context(), chat.ID, requestedPageSize(r))
	}
	if err != nil {
		h.internalError(w, "list messages", err)
		return
	}

	data, err := h.buildConversationData(r.Context(), chat, messages, hasMore, highlightID, false, requestLocale(r))
	if err != nil {
		h.internalError(w, "build conversation data", err)
		return
	}
	h.render(w, r, "messages-page.html", data)
}

func requestedPageSize(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		return 0
	}
	return n
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))

	var results []store.SearchResult
	if term != "" {
		var err error
		results, err = h.store.SearchMessages(r.Context(), chat.ID, term)
		if err != nil {
			h.internalError(w, "search messages", err)
			return
		}
	}

	h.render(w, r, "search-results.html", SearchData{Chat: chat, Term: term, Results: results})
}

func (h *Handler) calendar(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}

	month := r.URL.Query().Get("mes")
	if year, monthNum := r.URL.Query().Get("ano"), r.URL.Query().Get("mesNum"); year != "" && monthNum != "" {
		month = fmt.Sprintf("%s-%s", year, monthNum)
	}
	if month == "" {
		month = monthOf(chat.LastMessageAt)
	}
	if _, err := store.NextMonth(month); err != nil {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}

	days, err := h.store.DaysWithMessages(r.Context(), chat.ID, month)
	if err != nil {
		h.internalError(w, "count messages by day", err)
		return
	}
	h.render(w, r, "calendar.html", buildCalendar(chat, month, days, requestLocale(r)))
}

func (h *Handler) media(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	attachment, err := h.store.GetAttachment(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(h.cfg.MediaDir, filepath.FromSlash(attachment.StoredPath))
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		h.internalError(w, "stat media", err)
		return
	}

	// The export names its own attachments, and that name decides the extension the mime
	// was derived from: "recibo.html" arrives as text/html and would run as a page on this
	// app's own origin, with the whole archive one fetch away. Only what the UI renders
	// inline is served with its own type; the rest downloads.
	mediaType, inline := inlineMediaType(attachment.Mime)
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !inline {
		w.Header().Set("Content-Disposition", contentDisposition(attachment.Filename))
	}
	w.Header().Set("ETag", `"`+attachment.SHA256+`"`)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")

	http.ServeContent(w, r, attachment.Filename, info.ModTime(), file)
}

// "links" has no media_kind: it comes from text messages, not attachments.
var galleryTabs = []struct {
	Slug     string
	LabelKey string
	Kinds    []string
}{
	{"fotos", "gallery.tab_photos", []string{"image"}},
	{"videos", "gallery.tab_videos", []string{"video", "gif"}},
	{"figurinhas", "gallery.tab_stickers", []string{"sticker"}},
	{"audios", "gallery.tab_audio", []string{"audio", "voice"}},
	{"documentos", "gallery.tab_documents", []string{"document", "contact"}},
	{"links", "gallery.tab_links", nil},
}

// Without a cap a 10-year group asked the browser for 9084 images or 2781 audio players at
// once, and the tab took minutes.
const galleryPageSize = 60

// Same handler as a chat's own gallery: chat id 0 means "no chat filter" all the way down
// to the queries.
func (h *Handler) allMedia(w http.ResponseWriter, r *http.Request) {
	h.renderGallery(w, r, store.Chat{Name: locale.T(requestLocale(r), "gallery.all_chats")}, true)
}

func (h *Handler) gallery(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	h.renderGallery(w, r, chat, false)
}

func (h *Handler) renderGallery(w http.ResponseWriter, r *http.Request, chat store.Chat, global bool) {

	tab := r.URL.Query().Get("aba")
	if tab == "" {
		tab = galleryTabs[0].Slug
	}

	counts, err := h.store.CountAttachmentsByKind(r.Context(), chat.ID)
	if err != nil {
		h.internalError(w, "count attachments", err)
		return
	}

	// The links tab is the one kind that isn't an attachment: its count only exists after
	// extracting the URLs, and one message can carry several.
	messages, err := h.store.ListMessagesWithLink(r.Context(), chat.ID)
	if err != nil {
		h.internalError(w, "list links", err)
		return
	}
	links := buildLinks(messages)

	lang := requestLocale(r)
	data := GalleryData{Chat: chat, Tab: tab, Global: global}
	var kinds []string
	for _, a := range galleryTabs {
		active := a.Slug == tab
		if active {
			kinds = a.Kinds
		}
		n := 0
		if a.Kinds == nil {
			n = len(links)
		}
		for _, k := range a.Kinds {
			n += counts[k]
		}
		data.Tabs = append(data.Tabs, GalleryTab{Slug: a.Slug, Label: locale.T(lang, a.LabelKey), Active: active, Count: n})
	}

	if tab == "links" {
		data.Links = links
	} else {
		var beforeSentAt string
		var beforeID int64
		before := r.URL.Query().Get("before")
		if before != "" {
			sentAt, _, id, err := decodeCursor(before)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			beforeSentAt, beforeID = sentAt, id
		}

		attachments, err := h.store.ListAttachments(r.Context(), chat.ID, kinds, beforeSentAt, beforeID, galleryPageSize+1)
		if err != nil {
			h.internalError(w, "list attachments", err)
			return
		}
		if len(attachments) > galleryPageSize {
			last := attachments[galleryPageSize-1]
			data.HasMore = true
			data.NextCursor = encodeCursor(last.SentAt, 0, last.ID)
			attachments = attachments[:galleryPageSize]
		}
		data.Attachments = attachments
	}

	// A cursor means the sentinel asked for the next page: answer with the items alone,
	// which it swaps in place of itself.
	if r.URL.Query().Get("before") != "" {
		h.render(w, r, "gallery-items", data)
		return
	}
	h.render(w, r, "media-gallery.html", data)
}

func buildLinks(messages []store.Message) []LinkView {
	var links []LinkView
	for _, m := range messages {
		for _, u := range render.ExtractURLs(m.Body) {
			links = append(links, LinkView{
				URL:       u,
				Domain:    urlDomain(u),
				Snippet:   m.Body,
				SentAt:    m.SentAt,
				MessageID: m.ID,
			})
		}
	}
	return links
}

func urlDomain(u string) string {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return u
	}
	return strings.TrimPrefix(parsed.Host, "www.")
}

func (h *Handler) imports(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListImports(r.Context())
	if err != nil {
		h.internalError(w, "list imports", err)
		return
	}
	data := ImportsData{Imports: importViews(list, requestLocale(r))}
	if snapshot, ok := ingest.CurrentImport(); ok {
		data.Importing = true
		data.Running = progressView(snapshot, requestLocale(r))
	}
	if !isHTMXRequest(r) {
		h.renderFullPage(w, r, "imports.html", data)
		return
	}
	h.render(w, r, "imports.html", data)
}

// The unit changes with the phase: unzipping counts bytes, importing counts messages.
func progressView(s ingest.Snapshot, lang render.Locale) ProgressView {
	v := ProgressView{
		File:    s.File,
		Percent: s.Percent(),
		Elapsed: shortDuration(s.Elapsed),
	}
	// A scan with no import in flight: walking the inbox, or waiting out the gap between
	// the two passes. There is no total to measure against, so the bar runs indeterminate.
	if !s.Importing {
		v.Scanning = true
		v.Phase = locale.T(lang, "imports.phase_scanning")
		return v
	}
	switch s.Phase {
	case ingest.PhaseExtracting:
		v.Phase = locale.T(lang, "imports.phase_extracting")
		v.Detail = locale.T(lang, "imports.of", fileSize(s.Done), fileSize(s.Total))
	default:
		v.Phase = locale.T(lang, "imports.phase_importing")
		v.Detail = locale.T(lang, "imports.of_messages", s.Done, s.Total)
	}
	return v
}

func shortDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dmin%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// importsProgress answers the progress bar without touching the database — the import
// holds the only connection while its transaction is open.
func (h *Handler) importsProgress(w http.ResponseWriter, r *http.Request) {
	var data ImportsData
	if snapshot, ok := ingest.CurrentImport(); ok {
		data.Importing = true
		data.Running = progressView(snapshot, requestLocale(r))
	}
	h.render(w, r, "imports-progress.html", data)
}

// importViews turns the stored check codes into the sentences the panel shows. An unknown
// code (an older row, a newer binary) is skipped rather than printed raw.
func importViews(list []store.Import, lang render.Locale) []ImportView {
	views := make([]ImportView, len(list))
	for i, imp := range list {
		views[i] = ImportView{Import: imp}
		if imp.Checks == "" {
			continue
		}
		var checks []parser.Check
		if err := json.Unmarshal([]byte(imp.Checks), &checks); err != nil {
			continue
		}
		for _, c := range checks {
			key := "coherence." + c.Code
			// Line is absent for findings the ingest side adds, whose sentence takes only
			// the count; passing an unused argument would print Go's %!(EXTRA ...).
			args := []any{c.Count}
			if c.Line > 0 {
				args = append(args, c.Line)
			}
			if text := locale.T(lang, key, args...); text != key {
				views[i].Coherence = append(views[i].Coherence, text)
			}
		}
	}
	return views
}

func (h *Handler) importsBadge(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.CountImportsNeedingAttention(r.Context())
	if err != nil {
		h.internalError(w, "count pending imports", err)
		return
	}
	h.render(w, r, "imports-badge.html", ImportsBadgeData{Count: n, Imported: ingest.Imported()})
}

// Fires the scan in the background and answers with the progress bar: a scan takes seconds
// (two passes, §5.9) and minutes when there is something big to import, and the button used
// to sit there until it was done.
func (h *Handler) importsRescan(w http.ResponseWriter, r *http.Request) {
	if h.scanner == nil {
		h.imports(w, r)
		return
	}
	// The request's context ends with this response; the scan must outlive it.
	ctx := context.WithoutCancel(r.Context())
	// Marked here, not in the goroutine: the page rendered right below has to already show
	// the bar, and the goroutine may not have started yet.
	ingest.ScanStarted()
	go func() {
		defer ingest.ScanFinished()
		h.scanner.Scan(ctx)
	}()
	// The whole page, not just the bar: the list of what came in before stays on screen.
	h.imports(w, r)
}

func (h *Handler) setOwner(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	owner := strings.TrimSpace(r.FormValue("owner"))
	if owner == "" {
		http.Error(w, "owner required", http.StatusBadRequest)
		return
	}
	if err := h.store.SetOwner(r.Context(), chat.ID, owner); err != nil {
		h.internalError(w, "set owner", err)
		return
	}
	chat.Owner = &owner
	h.reloadConversation(w, r, chat)
}

func (h *Handler) renameChat(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if err := h.store.RenameChat(r.Context(), chat.ID, name); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "a chat with that name already exists", http.StatusConflict)
			return
		}
		h.internalError(w, "rename chat", err)
		return
	}
	chat.Name = name

	if _, sent := r.Form["phone"]; sent {
		phone := strings.TrimSpace(r.FormValue("phone"))
		if err := h.store.SetPhone(r.Context(), chat.ID, phone); err != nil {
			h.internalError(w, "set phone", err)
			return
		}
		chat.Phone = phone
	}

	h.reloadProfile(w, r, chat)
}

var allowedAvatarExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

const maxAvatarSize = 8 << 20 // 8 MiB

// The extension comes from the bytes, never from the name the client sent.
func (h *Handler) saveAvatarUpload(w http.ResponseWriter, r *http.Request, base string) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize)
	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		http.Error(w, "file too large or invalid", http.StatusBadRequest)
		return "", false
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return "", false
	}
	defer file.Close()

	sample := make([]byte, 512)
	n, err := file.Read(sample)
	if err != nil && err != io.EOF {
		h.internalError(w, "read avatar", err)
		return "", false
	}
	kind := http.DetectContentType(sample[:n])
	ext, ok := allowedAvatarExtensions[kind]
	if !ok {
		http.Error(w, "unsupported image format", http.StatusUnsupportedMediaType)
		return "", false
	}

	if err := os.MkdirAll(filepath.Join(h.cfg.MediaDir, "avatars"), 0o755); err != nil {
		h.internalError(w, "create avatars directory", err)
		return "", false
	}
	relativePath := fmt.Sprintf("avatars/%s%s", base, ext)
	dest, err := os.Create(filepath.Join(h.cfg.MediaDir, relativePath))
	if err != nil {
		h.internalError(w, "save avatar", err)
		return "", false
	}
	defer dest.Close()
	if _, err := dest.Write(sample[:n]); err != nil {
		h.internalError(w, "save avatar", err)
		return "", false
	}
	if _, err := io.Copy(dest, file); err != nil {
		h.internalError(w, "save avatar", err)
		return "", false
	}
	return relativePath, true
}

func (h *Handler) setAvatar(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	base := fmt.Sprint(chat.ID)
	sender := strings.TrimSpace(r.FormValue("sender"))
	if sender != "" {
		base = fmt.Sprintf("%d-%d", chat.ID, nameHash(sender))
	}
	relativePath, ok := h.saveAvatarUpload(w, r, base)
	if !ok {
		return
	}

	if sender != "" {
		if err := h.store.SetParticipantAvatarPath(r.Context(), chat.ID, sender, relativePath); err != nil {
			h.internalError(w, "record participant avatar", err)
			return
		}
		h.reloadAfterPhoto(w, r, chat)
		return
	}

	if err := h.store.SetAvatar(r.Context(), chat.ID, relativePath); err != nil {
		h.internalError(w, "record avatar", err)
		return
	}
	chat.AvatarPath = relativePath
	h.reloadProfile(w, r, chat)
}

// The user's own photo has no chat behind it, so the file itself is the state: one glob
// instead of a settings table for a single value.
func (h *Handler) hasMyPhoto() bool { return h.myPhotoPath() != "" }

func (h *Handler) myPhotoPath() string {
	matches, _ := filepath.Glob(filepath.Join(h.cfg.MediaDir, "avatars", "me.*"))
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func (h *Handler) myProfile(w http.ResponseWriter, r *http.Request) {
	saved, err := h.store.Setting(r.Context(), store.SettingMe)
	if err != nil {
		h.internalError(w, "read setting", err)
		return
	}
	data := MyProfileData{Photo: h.hasMyPhoto(), Name: saved}
	if saved == "" && h.cfg.Me != "" {
		data.Name, data.FromEnv = h.cfg.Me, true
	}
	h.render(w, r, "me.html", data)
}

func (h *Handler) saveMyProfile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if err := h.store.SetSetting(r.Context(), store.SettingMe, name); err != nil {
		h.internalError(w, "save setting", err)
		return
	}
	h.myProfile(w, r)
}

func (h *Handler) setMyPhoto(w http.ResponseWriter, r *http.Request) {
	for _, old := range func() []string { m, _ := filepath.Glob(filepath.Join(h.cfg.MediaDir, "avatars", "me.*")); return m }() {
		_ = os.Remove(old)
	}
	if _, ok := h.saveAvatarUpload(w, r, "me"); !ok {
		return
	}
	// The panel is the visible one; the rail's button follows out of band.
	h.renderWithOOB(w, r, "me.html", MyProfileData{Photo: true, Name: h.me(r.Context())},
		"my-photo-oob", MyPhotoData{Photo: true})
}

func (h *Handler) myPhoto(w http.ResponseWriter, r *http.Request) {
	path := h.myPhotoPath()
	if path == "" {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		h.internalError(w, "stat my photo", err)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func (h *Handler) removeAvatar(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	if sender := strings.TrimSpace(r.URL.Query().Get("sender")); sender != "" {
		if err := h.store.RemoveParticipantAvatar(r.Context(), chat.ID, sender); err != nil {
			h.internalError(w, "remove participant avatar", err)
			return
		}
		h.reloadAfterPhoto(w, r, chat)
		return
	}
	if err := h.store.SetAvatar(r.Context(), chat.ID, ""); err != nil {
		h.internalError(w, "remove avatar", err)
		return
	}
	chat.AvatarPath = ""
	h.reloadProfile(w, r, chat)
}

// A participant photo can also be borrowed from another chat instead of uploaded: the same
// person usually already has a 1:1 conversation here, with a photo on it.
func (h *Handler) participantPhotoPicker(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	sender := strings.TrimSpace(r.URL.Query().Get("sender"))
	if sender == "" {
		http.Error(w, "sender required", http.StatusBadRequest)
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	candidates, err := h.store.ChatsWithPhoto(r.Context(), term)
	if err != nil {
		h.internalError(w, "chats with photo", err)
		return
	}
	data := ParticipantPhotoData{
		Chat: chat, Sender: sender, Term: term, Candidates: candidates,
		FromModal:   r.URL.Query().Get("from") == "modal",
		Target:      "#right-panel",
		MembersTerm: r.URL.Query().Get("mq"),
	}
	if data.FromModal {
		data.From, data.Target = "modal", "#modal"
	}
	h.render(w, r, "participant-photo.html", data)
}

func (h *Handler) linkParticipantPhoto(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	sender := strings.TrimSpace(r.FormValue("sender"))
	linked, err := strconv.ParseInt(r.FormValue("chat_id"), 10, 64)
	if sender == "" || err != nil {
		http.Error(w, "sender and chat_id required", http.StatusBadRequest)
		return
	}
	if err := h.store.LinkParticipantAvatar(r.Context(), chat.ID, sender, linked); err != nil {
		h.internalError(w, "link participant avatar", err)
		return
	}
	h.reloadAfterPhoto(w, r, chat)
}

func (h *Handler) chatAvatar(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	if sender := strings.TrimSpace(r.URL.Query().Get("sender")); sender != "" {
		avatars, err := h.store.ParticipantAvatars(r.Context(), chat.ID)
		if err != nil {
			h.internalError(w, "participant avatar", err)
			return
		}
		chat.AvatarPath = avatars[sender].Path
	}
	if chat.AvatarPath == "" {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(h.cfg.MediaDir, filepath.FromSlash(chat.AvatarPath))
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		h.internalError(w, "stat avatar", err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	http.ServeContent(w, r, chat.AvatarPath, info.ModTime(), file)
}

func (h *Handler) contactProfile(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	data, err := h.buildProfileData(r.Context(), chat)
	if err != nil {
		h.internalError(w, "build profile data", err)
		return
	}
	h.render(w, r, "profile.html", data)
}

func (h *Handler) buildProfileData(ctx context.Context, chat store.Chat) (ProfileData, error) {
	senders, err := h.ownerCandidates(ctx, chat)
	if err != nil {
		return ProfileData{}, err
	}
	data := ProfileData{Chat: chat, OwnerCandidates: senders}

	counts, err := h.store.CountAttachmentsByKind(ctx, chat.ID)
	if err != nil {
		return ProfileData{}, err
	}
	for _, n := range counts {
		data.MediaCount += n
	}

	lists, err := h.store.ListLists(ctx)
	if err != nil {
		return ProfileData{}, err
	}
	if len(lists) > 0 {
		assoc, err := h.store.ListAssociations(ctx)
		if err != nil {
			return ProfileData{}, err
		}
		data.Lists = make([]TaggedList, len(lists))
		for i, l := range lists {
			data.Lists[i] = TaggedList{ID: l.ID, Name: l.Name, In: assoc[chat.ID][l.ID]}
		}
	}

	if chat.IsGroup {
		participants, err := h.groupParticipants(ctx, chat)
		if err != nil {
			return ProfileData{}, err
		}
		fillParticipants(&data, participants)
	}
	return data, nil
}

// One query for the whole chat; a group with no participant photo allocates nothing.
func (h *Handler) participantAvatarURLs(ctx context.Context, chat store.Chat) (map[string]string, error) {
	if !chat.IsGroup {
		return nil, nil
	}
	avatars, err := h.store.ParticipantAvatars(ctx, chat.ID)
	if err != nil {
		return nil, err
	}
	urls := make(map[string]string, len(avatars))
	for sender, a := range avatars {
		urls[sender] = participantAvatarURL(chat.ID, sender, a)
	}
	owner := h.me(ctx)
	if chat.Owner != nil {
		owner = *chat.Owner
	}
	if owner != "" && urls[owner] == "" && h.hasMyPhoto() {
		urls[owner] = "/eu/foto"
	}
	return urls, nil
}

// participantAvatarURL is empty when the participant has no photo, which renders initials.
// A linked photo is served by the other chat's own route — nothing to copy or resolve here.
func participantAvatarURL(chatID int64, sender string, a store.ParticipantAvatar) string {
	switch {
	case a.LinkedChatID != 0:
		return fmt.Sprintf("/chats/%d/avatar", a.LinkedChatID)
	case a.Path != "":
		return fmt.Sprintf("/chats/%d/avatar?sender=%s", chatID, url.QueryEscape(sender))
	}
	return ""
}

// Like the official app: the rest is behind "see all", which opens the searchable dialog.
const maxParticipantsShown = 10

func fillParticipants(data *ProfileData, all []ParticipantView) {
	data.ParticipantTotal = len(all)
	all = namedParticipantsFirst(all)
	data.ParticipantRest = 0
	if len(all) > maxParticipantsShown {
		data.ParticipantRest = len(all) - maxParticipantsShown
		all = all[:maxParticipantsShown]
	}
	data.Participants = all
}

// The owner, then saved contacts, then raw numbers, keeping the volume order inside each
// group: in a big group most senders are numbers the exporting phone never had saved, and
// the ten the panel shows should be the ten the user can recognise.
func namedParticipantsFirst(all []ParticipantView) []ParticipantView {
	ordered := slices.Clone(all)
	slices.SortStableFunc(ordered, func(a, b ParticipantView) int {
		return participantRank(a) - participantRank(b)
	})
	return ordered
}

func participantRank(p ParticipantView) int {
	switch {
	case p.IsOwner:
		return 0
	case parser.LooksLikePhoneNumber(p.Display):
		return 2
	default:
		return 1
	}
}

func matchesParticipant(p ParticipantView, needle string) bool {
	return needle == "" ||
		strings.Contains(strings.ToLower(p.Display), needle) ||
		strings.Contains(strings.ToLower(p.Original), needle)
}

// groupParticipantsByLetter builds the dialog's sections: letters first, then the "~" the
// export uses for a push name, then numbers — the official app's order.
func groupParticipantsByLetter(all []ParticipantView, term string) []ParticipantGroup {
	needle := strings.ToLower(term)
	matched := make([]ParticipantView, 0, len(all))
	for _, p := range all {
		if matchesParticipant(p, needle) {
			matched = append(matched, p)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return strings.ToLower(matched[i].Display) < strings.ToLower(matched[j].Display)
	})

	var groups []ParticipantGroup
	for _, p := range matched {
		letter := participantLetter(p.Display)
		if len(groups) == 0 || groups[len(groups)-1].Letter != letter {
			groups = append(groups, ParticipantGroup{Letter: letter})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, p)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return letterRank(groups[i].Letter) < letterRank(groups[j].Letter)
	})
	return groups
}

func participantLetter(name string) string {
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r == '~':
			return "~"
		case unicode.IsDigit(r), r == '+':
			return "#"
		case unicode.IsLetter(r):
			return strings.ToUpper(string(r))
		}
	}
	return "#"
}

func letterRank(letter string) int {
	switch letter {
	case "~":
		return 1
	case "#":
		return 2
	}
	return 0
}

// After a photo action, the answer goes back to wherever it came from: the members dialog
// when it was opened there, the profile panel otherwise.
func (h *Handler) reloadAfterPhoto(w http.ResponseWriter, r *http.Request, chat store.Chat) {
	if r.FormValue("from") != "modal" {
		h.reloadProfile(w, r, chat)
		return
	}
	all, err := h.groupParticipants(r.Context(), chat)
	if err != nil {
		h.internalError(w, "list participants", err)
		return
	}
	// "mq" comes from the photo picker, which uses "q" for its own search.
	term := strings.TrimSpace(r.FormValue("mq"))
	if term == "" {
		term = strings.TrimSpace(r.FormValue("q"))
	}
	h.render(w, r, "members.html", MembersData{
		Chat: chat, Term: term, Total: len(all), Groups: groupParticipantsByLetter(all, term),
	})
}

func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	all, err := h.groupParticipants(r.Context(), chat)
	if err != nil {
		h.internalError(w, "list participants", err)
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	h.render(w, r, "members.html", MembersData{
		Chat: chat, Term: term, Total: len(all), Groups: groupParticipantsByLetter(all, term),
	})
}

func (h *Handler) groupParticipants(ctx context.Context, chat store.Chat) ([]ParticipantView, error) {
	senders, err := h.store.SendersByVolume(ctx, chat.ID)
	if err != nil {
		return nil, err
	}
	nicknames, err := h.store.ChatNicknames(ctx, chat.ID)
	if err != nil {
		return nil, err
	}
	avatars, err := h.store.ParticipantAvatars(ctx, chat.ID)
	if err != nil {
		return nil, err
	}
	myPhoto := h.hasMyPhoto()
	myName := h.me(ctx)

	// Same resolution the conversation uses: the chat's own owner, or the name from your
	// profile when it was never picked here. Without this, a group where the owner is still
	// unknown showed nobody as "you" — and so no photo either.
	owner := h.me(ctx)
	if chat.Owner != nil {
		owner = *chat.Owner
	}

	participants := make([]ParticipantView, 0, len(senders))
	for _, s := range senders {
		if s == chat.Name {
			continue
		}
		p := ParticipantView{
			Original:  s,
			Display:   s,
			IsOwner:   s == owner && owner != "",
			AvatarURL: participantAvatarURL(chat.ID, s, avatars[s]),
		}
		// Your own photo and name stand for you in every group. A nickname set for that
		// sender still wins: it is the more specific choice.
		if p.IsOwner {
			if p.AvatarURL == "" && myPhoto {
				p.AvatarURL = "/eu/foto"
			}
			if myName != "" && !p.Renamed {
				p.Display, p.Renamed = myName, myName != p.Original
			}
		}
		if nickname := nicknames[s]; nickname != "" {
			p.Display = nickname
			p.Renamed = true
		}
		participants = append(participants, p)
	}
	return participants, nil
}

// An empty name removes the nickname and goes back to showing the export's.
func (h *Handler) renameParticipant(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	sender := strings.TrimSpace(r.FormValue("sender"))
	if sender == "" {
		http.Error(w, "sender required", http.StatusBadRequest)
		return
	}
	if err := h.store.SetNickname(r.Context(), chat.ID, sender, strings.TrimSpace(r.FormValue("name"))); err != nil {
		h.internalError(w, "rename participant", err)
		return
	}
	profileData, err := h.buildProfileData(r.Context(), chat)
	if err != nil {
		h.internalError(w, "build profile", err)
		return
	}
	messages, hasMore, err := h.store.LastMessagePage(r.Context(), chat.ID, requestedPageSize(r))
	if err != nil {
		h.internalError(w, "list messages", err)
		return
	}
	conversationData, err := h.buildConversationData(r.Context(), chat, messages, hasMore, 0, true, requestLocale(r))
	if err != nil {
		h.internalError(w, "build conversation data", err)
		return
	}
	h.renderWithOOB(w, r, "profile.html", profileData, "conversation-oob", conversationData)
}

func (h *Handler) getChatOrNotFound(w http.ResponseWriter, r *http.Request) (store.Chat, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return store.Chat{}, false
	}
	chat, err := h.store.GetChat(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return store.Chat{}, false
	}
	return chat, true
}

func (h *Handler) reloadConversation(w http.ResponseWriter, r *http.Request, chat store.Chat) {
	messages, hasMore, err := h.store.LastMessagePage(r.Context(), chat.ID, requestedPageSize(r))
	if err != nil {
		h.internalError(w, "list messages", err)
		return
	}
	data, err := h.buildConversationData(r.Context(), chat, messages, hasMore, 0, true, requestLocale(r))
	if err != nil {
		h.internalError(w, "build conversation data", err)
		return
	}
	h.render(w, r, "conversation.html", data)
}

func (h *Handler) reloadProfile(w http.ResponseWriter, r *http.Request, chat store.Chat) {
	data, err := h.buildProfileData(r.Context(), chat)
	if err != nil {
		h.internalError(w, "build profile data", err)
		return
	}
	h.renderWithOOB(w, r, "profile.html", data, "header-oob", data)
}

// me is the sender treated as the user: what the profile screen set, or VAULTZAP_ME when
// it was never set. The setting wins so a change in the UI does not need a restart.
func (h *Handler) me(ctx context.Context) string {
	if value, err := h.store.Setting(ctx, store.SettingMe); err == nil && value != "" {
		return value
	}
	return h.cfg.Me
}

func (h *Handler) buildConversationData(ctx context.Context, chat store.Chat, messages []store.Message, hasMore bool, highlightID int64, withHeader bool, lang render.Locale) (ConversationData, error) {
	owner := h.me(ctx)
	if chat.Owner != nil {
		owner = *chat.Owner
	}

	cursor := ""
	if hasMore && len(messages) > 0 {
		cursor = encodeCursor(messages[0].SentAt, messages[0].Seq, messages[0].ID)
	}

	nicknames, err := h.store.ChatNicknames(ctx, chat.ID)
	if err != nil {
		return ConversationData{}, err
	}

	avatars, err := h.participantAvatarURLs(ctx, chat)
	if err != nil {
		return ConversationData{}, err
	}

	data := ConversationData{
		Chat: chat,
		Messages: buildMessageViews(messages, conversationContext{
			Owner:      owner,
			IsGroup:    chat.IsGroup,
			ChatID:     chat.ID,
			AvatarPath: chat.AvatarPath,
			Nicknames:  nicknames,
			Avatars:    avatars,
			OwnerName:  h.me(ctx),
			Locale:     lang,
		}, time.Now().In(h.tz), highlightID),
		HasMore:    hasMore,
		NextCursor: cursor,
	}

	if withHeader {
		senders, err := h.ownerCandidates(ctx, chat)
		if err != nil {
			return ConversationData{}, err
		}
		data.OwnerCandidates, data.OwnerOthers = splitCandidates(senders)
		data.ShowOwnerPicker = chat.Owner == nil && !senderInList(senders, h.me(ctx))

		// Only the full conversation carries the pinned strip; a plain page of messages
		// swaps into #list-messages, which the strip sits outside of.
		pinned, err := h.store.ListPinnedMessages(ctx, chat.ID)
		if err != nil {
			return ConversationData{}, err
		}
		data.PinnedMessages = pinned
	}

	return data, nil
}

// Filtered on the server because a group can have hundreds: shipping them all as hidden
// buttons and filtering in the browser is what the dropdown used to do.
func (h *Handler) senders(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	all, err := h.ownerCandidates(r.Context(), chat)
	if err != nil {
		h.internalError(w, "list senders", err)
		return
	}

	term := strings.TrimSpace(r.URL.Query().Get("q"))
	needle := strings.ToLower(term)
	senders := make([]string, 0, len(all))
	for _, s := range namedFirst(all) {
		if needle == "" || strings.Contains(strings.ToLower(s), needle) {
			senders = append(senders, s)
		}
	}

	h.render(w, r, "senders.html", SendersData{Chat: chat, Term: term, Senders: senders})
}

func (h *Handler) ownerCandidates(ctx context.Context, chat store.Chat) ([]string, error) {
	senders, err := h.store.SendersByVolume(ctx, chat.ID)
	if err != nil {
		return nil, err
	}
	if !chat.IsGroup {
		return senders, nil
	}
	candidates := make([]string, 0, len(senders))
	for _, s := range senders {
		if s != chat.Name {
			candidates = append(candidates, s)
		}
	}
	return candidates, nil
}

// maxOwnerChips is how many senders the bar shows as buttons. Beyond that the list stops
// being scannable — a group with 725 senders filled the whole screen and pushed the
// conversation off it. The rest goes into a select, which is searchable by typing.
const maxOwnerChips = 8

func splitCandidates(senders []string) (chips, others []string) {
	ordered := namedFirst(senders)
	if len(ordered) <= maxOwnerChips {
		return ordered, nil
	}
	// The picker still carries everyone, including the ones already shown as chips, so
	// there is one place that always has the answer.
	return ordered[:maxOwnerChips], ordered
}

// Saved contacts before raw numbers: someone looking for themselves recognises a name, and
// scrolling past hundreds of numbers to find one is the whole complaint.
func namedFirst(senders []string) []string {
	named := make([]string, 0, len(senders))
	numbers := make([]string, 0, len(senders))
	for _, s := range senders {
		if parser.LooksLikePhoneNumber(s) {
			numbers = append(numbers, s)
			continue
		}
		named = append(named, s)
	}
	return append(named, numbers...)
}

func senderInList(senders []string, target string) bool {
	if target == "" {
		return false
	}
	for _, s := range senders {
		if s == target {
			return true
		}
	}
	return false
}

func encodeCursor(sentAt string, seq int, id int64) string {
	return url.QueryEscape(fmt.Sprintf("%s,%d,%d", sentAt, seq, id))
}

// A two-part cursor (the format before message id became the tiebreak) is still
// accepted, with id 0 — no row has an id below 1, so it reproduces the old
// "strictly before (sent_at, seq)" behaviour instead of 400ing an open tab.
func decodeCursor(value string) (sentAt string, seq int, id int64, err error) {
	parts := strings.Split(value, ",")
	if len(parts) < 2 || len(parts) > 3 {
		return "", 0, 0, fmt.Errorf("invalid cursor format: %q", value)
	}
	sentAt = parts[0]
	seq, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid seq in cursor: %w", err)
	}
	if len(parts) == 3 {
		id, err = strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return "", 0, 0, fmt.Errorf("invalid id in cursor: %w", err)
		}
	}
	return sentAt, seq, id, nil
}

func loadTemplates(lang render.Locale) (*template.Template, error) {
	base, err := baseTemplates()
	if err != nil {
		return nil, err
	}
	clone, err := base.Clone()
	if err != nil {
		return nil, err
	}
	return clone.Funcs(localeTemplateFuncs(lang)), nil
}

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// Used by routes that are normally an htmx fragment target (hx-push-url) when
// they are loaded directly: there is no #conversation in the DOM to swap into.
func (h *Handler) renderFullPage(w http.ResponseWriter, r *http.Request, contentTemplate string, contentData any) {
	lang := requestLocale(r)
	templates, err := loadTemplates(lang)
	if err != nil {
		h.internalError(w, "load templates", err)
		return
	}
	chatList, err := h.buildChatList(r.Context(), store.ChatFilter{}, lang)
	if err != nil {
		h.internalError(w, "list chats", err)
		return
	}
	var content bytes.Buffer
	if err := templates.ExecuteTemplate(&content, contentTemplate, contentData); err != nil {
		h.internalError(w, "render content", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := PageData{ChatList: chatList, Content: template.HTML(content.String())}
	if err := templates.ExecuteTemplate(w, "layout.html", page); err != nil {
		slog.Error("render template", "template", "layout.html", "error", err)
	}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, templateName string, data any) {
	templates, err := loadTemplates(requestLocale(r))
	if err != nil {
		h.internalError(w, "load templates", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, templateName, data); err != nil {
		slog.Error("render template", "template", templateName, "error", err)
	}
}

func (h *Handler) renderWithOOB(w http.ResponseWriter, r *http.Request, main string, mainData any, oob string, oobData any) {
	templates, err := loadTemplates(requestLocale(r))
	if err != nil {
		h.internalError(w, "load templates", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, main, mainData); err != nil {
		slog.Error("render template", "template", main, "error", err)
		return
	}
	if err := templates.ExecuteTemplate(w, oob, oobData); err != nil {
		slog.Error("render template", "template", oob, "error", err)
	}
}

func (h *Handler) internalError(w http.ResponseWriter, context string, err error) {
	slog.Error(context, "error", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

const maxPinnedChats = 3

func (h *Handler) pinChat(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}

	if !chat.Pinned {
		pinned, err := h.store.CountPinned(r.Context())
		if err != nil {
			h.internalError(w, "count pinned", err)
			return
		}
		if pinned >= maxPinnedChats {
			h.triggerEvent(w, "alert", locale.T(requestLocale(r), "toast.pin_limit", maxPinnedChats))
			h.respondChatList(w, r)
			return
		}
	}

	if err := h.store.SetPinned(r.Context(), chat.ID, !chat.Pinned); err != nil {
		h.internalError(w, "pin chat", err)
		return
	}

	text := locale.T(requestLocale(r), "toast.chat_pinned")
	if chat.Pinned {
		text = locale.T(requestLocale(r), "toast.chat_unpinned")
	}
	h.notify(w, r, toast{
		Text: text,
		Undo: fmt.Sprintf("/chats/%d/fixar%s", chat.ID, filterQuery(r)),
	})
	h.respondChatList(w, r)
}

func (h *Handler) favoriteChat(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	if err := h.store.SetFavorite(r.Context(), chat.ID, !chat.Favorite); err != nil {
		h.internalError(w, "favorite chat", err)
		return
	}

	// Asymmetry copied from the official app: REMOVING offers "Desfazer", ADDING
	// does not — adding is self-evident and one click from being reversed.
	// chat.Favorite still holds the state from BEFORE the toggle above.
	lang := requestLocale(r)
	if chat.Favorite {
		t := toast{
			Text: locale.T(lang, "toast.chat_unfavorited"),
			Undo: fmt.Sprintf("/chats/%d/favoritar%s", chat.ID, filterQuery(r)),
		}
		// From the profile panel the answer is profile.html, so the undo has to land
		// there, not on the list.
		if r.FormValue("from") == "perfil" {
			t.Target = "#right-panel"
		}
		h.notify(w, r, t)
	} else {
		h.notify(w, r, toast{Text: locale.T(lang, "toast.chat_favorited")})
	}

	if h.respondFromProfile(w, r, chat.ID) {
		return
	}
	h.respondChatList(w, r)
}

func (h *Handler) chatInList(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}

	listID, _ := strconv.ParseInt(r.FormValue("list_id"), 10, 64)
	if listID == 0 {
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "list_id or name required", http.StatusBadRequest)
			return
		}
		newID, err := h.store.CreateList(r.Context(), name)
		if err != nil {
			http.Error(w, "a list with that name already exists", http.StatusConflict)
			return
		}
		listID = newID
	}

	in := r.FormValue("in") != "0"
	if err := h.store.SetChatInList(r.Context(), chat.ID, listID, in); err != nil {
		h.internalError(w, "set chat in list", err)
		return
	}

	text := locale.T(requestLocale(r), "toast.removed_from_list")
	if in {
		text = locale.T(requestLocale(r), "toast.added_to_list")
	}
	h.notify(w, r, toast{
		Text:   text,
		Undo:   fmt.Sprintf("/chats/%d/listas%s", chat.ID, filterQuery(r)),
		Values: map[string]string{"list_id": strconv.FormatInt(listID, 10), "in": invert(in)},
	})
	if h.respondFromProfile(w, r, chat.ID) {
		return
	}
	h.respondChatList(w, r)
}

// Reports whether it handled the response — a request from anywhere else falls
// through to the caller's plain chat-list answer.
func (h *Handler) respondFromProfile(w http.ResponseWriter, r *http.Request, chatID int64) bool {
	if r.FormValue("from") != "perfil" {
		return false
	}
	chat, err := h.store.GetChat(r.Context(), chatID)
	if err != nil {
		h.internalError(w, "get chat", err)
		return true
	}
	profile, err := h.buildProfileData(r.Context(), chat)
	if err != nil {
		h.internalError(w, "build profile data", err)
		return true
	}
	list, err := h.buildChatList(r.Context(), filterFromURL(r), requestLocale(r))
	if err != nil {
		h.internalError(w, "list chats", err)
		return true
	}
	h.renderWithOOB(w, r, "profile.html", profile, "chat-list-oob", list)
	return true
}

// Undo, when set, is the URL the "Undo" action POSTs to, with Values in the body.
// Target/Swap let it land somewhere other than the chat list; empty keeps the
// chat-list default.
type toast struct {
	Text   string            `json:"text"`
	Undo   string            `json:"undo,omitempty"`
	Target string            `json:"target,omitempty"`
	Swap   string            `json:"swap,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

func invert(b bool) string {
	if b {
		return "0"
	}
	return "1"
}

func filterQuery(r *http.Request) string {
	if q := r.URL.RawQuery; q != "" {
		return "?" + q
	}
	return ""
}

// Skipped when the request is itself an undo, or each undo would open another
// toast offering another undo.
func (h *Handler) notify(w http.ResponseWriter, r *http.Request, t toast) {
	if r.FormValue("undoing") == "1" {
		return
	}
	h.triggerEvent(w, "toast", t)
}

func (h *Handler) triggerEvent(w http.ResponseWriter, name string, payload any) {
	body, err := json.Marshal(map[string]any{name: payload})
	if err != nil {
		slog.Error("serialize htmx event", "event", name, "error", err)
		return
	}
	w.Header().Set("HX-Trigger", escapeNonASCII(string(body)))
}

// HTTP headers are read as latin-1, so non-ASCII in the HX-Trigger JSON has to
// travel escaped.
func escapeNonASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < utf8.RuneSelf {
			b.WriteRune(r)
			continue
		}
		if r1, r2 := utf16.EncodeRune(r); r1 != utf8.RuneError {
			fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
			continue
		}
		fmt.Fprintf(&b, `\u%04x`, r)
	}
	return b.String()
}

func (h *Handler) createList(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if _, err := h.store.CreateList(r.Context(), name); err != nil {
		http.Error(w, "a list with that name already exists", http.StatusConflict)
		return
	}
	h.respondChatList(w, r)
}

func (h *Handler) deleteList(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := h.store.DeleteList(r.Context(), id); err != nil {
		h.internalError(w, "delete list", err)
		return
	}
	r.URL.RawQuery = ""
	h.respondChatList(w, r)
}

func (h *Handler) updateConversation(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	units, err := ingest.ListInboxUnits(h.cfg.Inbox)
	if err != nil {
		h.internalError(w, "list inbox", err)
		return
	}
	// With the default "move" policy the file is no longer in the inbox, so listing only
	// the inbox left the panel empty exactly when the user clicks "Reimport…".
	imported, err := ingest.ListImportedUnits(h.cfg.ImportedDir)
	if err != nil {
		h.internalError(w, "list imported", err)
		return
	}
	h.render(w, r, "update-conversation.html", UpdateData{
		Chat: chat, Units: units, Imported: imported, Inbox: h.cfg.Inbox,
	})
}

// Imports the chosen unit into this conversation, ignoring the file name — which
// is the whole point: the scan matches export to chat by name.
func (h *Handler) applyUpdate(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.FormValue("file"))
	if name == "" {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	// Two origins, two guards. The inbox only ever offers top-level names, so anything
	// with a separator is refused outright; the archived copies live under
	// <imported>/YYYY-MM/, and ResolveImported is what keeps the name inside that folder.
	var path string
	if r.FormValue("origem") == "imported" {
		resolved, err := ingest.ResolveImported(h.cfg.ImportedDir, name)
		if err != nil {
			http.Error(w, "invalid file name", http.StatusBadRequest)
			return
		}
		path = resolved
	} else {
		if name != filepath.Base(name) {
			http.Error(w, "invalid file name", http.StatusBadRequest)
			return
		}
		path = filepath.Join(h.cfg.Inbox, name)
	}
	// In the background: a 7 GB export takes minutes, and in the handler that left the
	// browser hanging. WithoutCancel because the request is about to end and would cancel
	// the import with it — which is safe to redo, the import is idempotent.
	go func(ctx context.Context) {
		report, err := h.scanner.ImportInto(ctx, path, name, chat.ID)
		if err != nil {
			slog.Error("update conversation", "chat_id", chat.ID, "file", name, "error", err)
			return
		}
		slog.Info("conversation updated", "chat_id", chat.ID, "file", name,
			"added", report.Added, "skipped", report.Skipped)
	}(context.WithoutCancel(r.Context()))

	units, err := ingest.ListInboxUnits(h.cfg.Inbox)
	if err != nil {
		h.internalError(w, "list inbox", err)
		return
	}
	imported, err := ingest.ListImportedUnits(h.cfg.ImportedDir)
	if err != nil {
		h.internalError(w, "list imported", err)
		return
	}
	h.render(w, r, "update-conversation.html", UpdateData{
		Chat: chat, Units: units, Imported: imported, Inbox: h.cfg.Inbox, Started: name,
	})
}

func (h *Handler) mergePicker(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	candidates, err := h.store.SearchMergeCandidates(r.Context(), chat.ID, chat.IsGroup, term)
	if err != nil {
		h.internalError(w, "search merge candidates", err)
		return
	}
	h.render(w, r, "merge-picker.html", MergeData{Chat: chat, Term: term, Candidates: candidates})
}

// The current chat stays, the chosen one disappears. Irreversible.
func (h *Handler) mergeChats(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	source, err := strconv.ParseInt(r.FormValue("origem"), 10, 64)
	if err != nil {
		http.Error(w, "origem required", http.StatusBadRequest)
		return
	}

	report, err := h.store.MergeChats(r.Context(), chat.ID, source)
	if err != nil {
		slog.Error("merge chats", "dest", chat.ID, "source", source, "error", err)
		h.triggerEvent(w, "alert", locale.T(requestLocale(r), "toast.merge_error", err.Error()))
		h.mergePicker(w, r)
		return
	}

	slog.Info("chats merged", "dest", chat.ID, "source", source,
		"messages_moved", report.MessagesMoved, "messages_duplicated", report.MessagesDuplicated)
	h.notify(w, r, toast{Text: locale.T(requestLocale(r), "toast.merge_result",
		report.MessagesMoved, report.MessagesDuplicated)})

	messages, hasMore, err := h.store.LastMessagePage(r.Context(), chat.ID, requestedPageSize(r))
	if err != nil {
		h.internalError(w, "list messages", err)
		return
	}
	conversationData, err := h.buildConversationData(r.Context(), chat, messages, hasMore, 0, true, requestLocale(r))
	if err != nil {
		h.internalError(w, "build conversation data", err)
		return
	}
	h.renderWithOOB(w, r, "right-panel-empty", nil, "conversation-oob", conversationData)
}

func (h *Handler) exportConversation(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}

	format := export.Format(r.URL.Query().Get("formato"))
	if format == "" {
		format = export.TXT
	}
	if !slices.Contains(export.Formats, format) {
		http.Error(w, "unknown export format", http.StatusBadRequest)
		return
	}

	owner := h.me(r.Context())
	if chat.Owner != nil {
		owner = *chat.Owner
	}
	nicknames, err := h.store.ChatNicknames(r.Context(), chat.ID)
	if err != nil {
		h.internalError(w, "load nicknames", err)
		return
	}
	options := export.Options{Owner: owner, Nicknames: nicknames, Locale: requestLocale(r)}

	fileName := export.FileName(chat, format)
	w.Header().Set("Content-Type", format.ContentType())
	w.Header().Set("Content-Disposition", contentDisposition(fileName))

	if err := export.Export(r.Context(), h.store, chat, options, format, w); err != nil {
		slog.Error("export conversation", "chat_id", chat.ID, "format", format, "error", err)
	}
}

// ASCII fallback name (filename) plus the real UTF-8 name (filename*, RFC 6266).
// inlineMediaType keeps only the families the conversation actually renders in place.
// SVG is left out on purpose: it is an image to <img>, but a scriptable document when the
// browser opens it directly.
func inlineMediaType(mediaType string) (string, bool) {
	base, _, _ := strings.Cut(mediaType, ";")
	base = strings.TrimSpace(strings.ToLower(base))
	switch {
	case base == "image/svg+xml", base == "image/svg":
	case strings.HasPrefix(base, "image/"),
		strings.HasPrefix(base, "video/"),
		strings.HasPrefix(base, "audio/"):
		return mediaType, true
	}
	return "application/octet-stream", false
}

func contentDisposition(fileName string) string {
	asciiReplacer := strings.NewReplacer("\"", "'")
	asciiOnly := true
	for _, r := range fileName {
		if r > 127 {
			asciiOnly = false
			break
		}
	}
	fallback := "conversa" + filepath.Ext(fileName)
	if asciiOnly {
		fallback = asciiReplacer.Replace(fileName)
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		asciiReplacer.Replace(fallback), url.PathEscape(fileName))
}

func (h *Handler) favoriteMessages(w http.ResponseWriter, r *http.Request) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	messages, err := h.store.ListFavoriteMessages(r.Context(), chat.ID)
	if err != nil {
		h.internalError(w, "list favorite messages", err)
		return
	}

	owner := h.me(r.Context())
	if chat.Owner != nil {
		owner = *chat.Owner
	}
	nicknames, err := h.store.ChatNicknames(r.Context(), chat.ID)
	if err != nil {
		h.internalError(w, "chat nicknames", err)
		return
	}
	// buildMessageViews also computes grouping and date dividers, meaningless in a
	// list plucked from all over the conversation; the template ignores them.
	views := buildMessageViews(messages, conversationContext{
		Owner:     owner,
		IsGroup:   chat.IsGroup,
		ChatID:    chat.ID,
		Nicknames: nicknames,
		Locale:    requestLocale(r),
	}, time.Now().In(h.tz), 0)

	h.render(w, r, "favorite-messages.html", MessageJumpListData{Chat: chat, Messages: views})
}

// Both answer with the message-actions fragment alone: re-rendering the whole
// bubble would need its neighbours to recompute grouping.
func (h *Handler) toggleMessageFavorite(w http.ResponseWriter, r *http.Request) {
	h.toggleMessageFlag(w, r, h.store.ToggleMessageFavorite, false,
		"toast.message_starred", "toast.message_unstarred")
}

// No toast when pinning a MESSAGE, by explicit request — the pinned strip under
// the header already shows it. The toast is for pinning a CONVERSATION.
func (h *Handler) toggleMessagePinned(w http.ResponseWriter, r *http.Request) {
	h.toggleMessageFlag(w, r, h.store.TogglePinnedMessage, true, "", "")
}

// refreshBanner is set by the pinned toggle: the strip lives outside this
// fragment's target, so it comes back as an out-of-band swap.
// onKey/offKey are the toast's locale keys; both empty means no toast.
func (h *Handler) toggleMessageFlag(w http.ResponseWriter, r *http.Request, toggle func(context.Context, int64, int64) (favorite, pinned bool, sentAt string, err error), refreshBanner bool, onKey, offKey string) {
	chat, ok := h.getChatOrNotFound(w, r)
	if !ok {
		return
	}
	messageID, err := strconv.ParseInt(r.PathValue("messageID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}

	favorite, pinned, sentAt, err := toggle(r.Context(), chat.ID, messageID)
	if err != nil {
		// No row matched: the id is not in this chat, or it is a system message.
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.internalError(w, "toggle message flag", err)
		return
	}

	shortTime := ""
	if t, err := time.Parse("2006-01-02 15:04:05", sentAt); err == nil {
		shortTime = render.ClockTime(t, requestLocale(r))
	}
	wrapClass := "meta-bubble"
	if r.FormValue("sticker") == "true" {
		wrapClass = "meta-sticker"
	}
	actions := MessageActionsData{
		ChatID: chat.ID, MessageID: messageID,
		Favorite: favorite, Pinned: pinned,
		ShortTime: shortTime, WrapClass: wrapClass,
	}

	// Must be set before any render: notify writes an HTTP header, and headers are
	// gone once the body starts.
	if onKey != "" {
		changed := favorite
		if refreshBanner {
			changed = pinned
		}
		key := offKey
		if changed {
			key = onKey
		}
		h.notify(w, r, toast{
			Text: locale.T(requestLocale(r), key),
			// Taken from r.URL.Path instead of spelled out, so it cannot drift from
			// whichever route (favoritar/fixar) actually got called.
			Undo:   r.URL.Path,
			Target: fmt.Sprintf("#meta-%d", messageID),
			Swap:   "outerHTML",
			Values: map[string]string{"sticker": r.FormValue("sticker")},
		})
	}

	if refreshBanner {
		pinnedList, err := h.store.ListPinnedMessages(r.Context(), chat.ID)
		if err != nil {
			h.internalError(w, "list pinned messages", err)
			return
		}
		h.renderWithOOB(w, r, "message-actions", actions, "pinned-banner-oob",
			ConversationData{Chat: chat, PinnedMessages: pinnedList})
		return
	}

	h.render(w, r, "message-actions", actions)
}
