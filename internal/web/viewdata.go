package web

import (
	"html/template"

	"github.com/wallacepnts/vaultzap/internal/ingest"
	"github.com/wallacepnts/vaultzap/internal/render"
	"github.com/wallacepnts/vaultzap/internal/store"
)

// PageData feeds layout.html for a full page load (direct navigation, refresh, bookmark).
// Content is the pre-rendered fragment that would normally go straight into #conversation.
type PageData struct {
	ChatList ChatListData
	Content  template.HTML
}

type ChatListData struct {
	Chats         []ChatView
	Locale        render.Locale
	Search        string
	Archived      bool
	TotalArchived int

	// ActiveFilter is "tudo"|"favoritas"|"grupos"; ActiveList (> 0) marks a user list's chip.
	ActiveFilter   string
	ActiveList     int64
	TotalFavorites int
	TotalGroups    int
	Lists          []store.List
	// Photo is the user's own picture, shown at the bottom of the sidebar.
	Photo bool
	// Version is the build's own version, shown under the source-code link.
	Version string
}

// NewListData feeds new-list.html: every chat to pick from, which of them start out
// included, and — after a name collision — the message plus what the user had typed and
// picked, so the panel comes back filled in instead of blank.
type NewListData struct {
	Chats       []ChatView
	Preselected map[int64]bool
	Name        string
	Error       string
}

type ConversationData struct {
	Chat       store.Chat
	Messages   []MessageView
	HasMore    bool
	NextCursor string

	// ShowOwnerPicker / OwnerCandidates feed the "who am I" bar, populated when owner is
	// null. OwnerCandidates holds only the busiest senders — a 10-year group has hundreds,
	// and rendering all of them as chips buried the conversation under a wall of numbers.
	// OwnerOthers carries the full list for the picker beside them, empty when they fit.
	ShowOwnerPicker bool
	OwnerCandidates []string
	OwnerOthers     []string

	// PinnedMessages feeds the strip under the header. Empty renders no strip at all.
	PinnedMessages []store.Message
}

// GalleryData feeds media-gallery.html. Only one list is populated at a time: the
// "links" tab uses Links, every other tab uses Attachments.
type GalleryData struct {
	Chat        store.Chat
	Attachments []store.Attachment
	Links       []LinkView
	Tabs        []GalleryTab
	Tab         string
	// Global drops the chat header and points the tabs at /midia.
	Global bool

	// One page at a time, like the conversation: NextCursor feeds the sentinel at the end.
	HasMore    bool
	NextCursor string
}

type GalleryTab struct {
	Slug   string
	Label  string
	Active bool
	Count  int
}

type LinkView struct {
	URL       string
	Domain    string
	Snippet   string
	SentAt    string
	MessageID int64
}

type ParticipantView struct {
	// Original is the sender exactly as the export has it — what goes in the POST and matches
	// messages.sender; Display is what shows on screen.
	Original  string
	Display   string
	Renamed   bool
	AvatarURL string
	// IsOwner marks the chat owner: listed like the official app does, but not renameable —
	// the owner is a name the user already chose, not a contact to relabel.
	IsOwner bool
}

// Also feeds header-oob.html, which keeps #conversation's header in sync after a rename.
type ProfileData struct {
	Chat            store.Chat
	OwnerCandidates []string
	// Participants is only populated for a group, and only up to maxParticipantsShown —
	// ParticipantTotal is how many there really are.
	Participants     []ParticipantView
	ParticipantTotal int
	// ParticipantRest is how many did not fit, behind the "see all" button.
	ParticipantRest int
	Lists           []TaggedList
	MediaCount      int
}

type UpdateData struct {
	Chat  store.Chat
	Units []ingest.InboxUnit
	// Imported is what the "move" policy archived; empty with the "keep" policy.
	Imported []ingest.InboxUnit
	Inbox    string
	// Report is only populated right after an import.
	Report *ingest.Report
	// Started names the unit whose import was just kicked off; the panel then shows the
	// progress bar instead of the result, which only exists when the import ends.
	Started string
}

// ParticipantPhotoData feeds the panel that borrows another chat's photo for a participant.
type ParticipantPhotoData struct {
	Chat       store.Chat
	Sender     string
	Term       string
	Candidates []store.Chat
	// FromModal picks the dialog shell over the right panel; Target/From carry that choice
	// into the fragment's own requests.
	FromModal bool
	From      string
	Target    string
	// MembersTerm is the dialog's own search term, carried through so going back lands on
	// the same filtered list instead of the top of 725 rows.
	MembersTerm string
}

type ParticipantGroup struct {
	Letter string
	Items  []ParticipantView
}

// MembersData feeds members.html, the "search members" dialog.
type MembersData struct {
	Chat   store.Chat
	Term   string
	Total  int
	Groups []ParticipantGroup
}

type MergeData struct {
	Chat       store.Chat
	Term       string
	Candidates []store.Chat
}

// SendersData feeds senders.html: the whole sender list, filtered on the server.
type SendersData struct {
	Chat    store.Chat
	Term    string
	Senders []string
}

type SearchData struct {
	Chat    store.Chat
	Term    string
	Results []store.SearchResult
}

// ImportView adds to a stored import what only the request can build: the coherence
// findings, translated. The parser stores codes because it has no notion of locale (§11.14).
type ImportView struct {
	store.Import
	Coherence []string
}

type ImportsData struct {
	Imports []ImportView

	// Running describes the import in flight; Importing is false when the folder is idle.
	Importing bool
	Running   ProgressView
}

// ProgressView is the progress bar's data, already formatted for the template.
type ProgressView struct {
	// Scanning marks the walk itself, with no file being imported yet.
	Scanning bool
	File     string
	Phase    string // texto já traduzido
	Percent  int
	Detail   string // "3,2 GB de 7,0 GB" ou "12.480 de 98.009 mensagens"
	Elapsed  string // "2min14s"
}

// MyPhotoData feeds the rail's own-photo button. Photo is false when none was uploaded.
type MyPhotoData struct {
	Photo bool
}

// MyProfileData feeds me.html. FromEnv marks a name coming from VAULTZAP_ME, never saved
// here, so the panel can say where it came from.
type MyProfileData struct {
	Photo   bool
	Name    string
	FromEnv bool
}

type ImportsBadgeData struct {
	Count int
	// Imported changes whenever an import brought messages in; app.js watches it to refresh
	// the chat list from any screen.
	Imported int64
}

// Messages plucked from all over the conversation, each jumping to its place via "?around=".
type MessageJumpListData struct {
	Chat     store.Chat
	Messages []MessageView
}

// MessageActionsData feeds the "message-actions" fragment (star + pin badges and the
// dropdown that toggles them). The POST handlers answer with it re-rendered.
type MessageActionsData struct {
	ChatID    int64
	MessageID int64
	Favorite  bool
	Pinned    bool
	ShortTime string
	WrapClass string // "meta-bubble" or "meta-sticker"
}
