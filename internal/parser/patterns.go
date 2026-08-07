// Single table of locale-dependent patterns.
package parser

import (
	"path/filepath"
	"regexp"
	"strings"
)

var iosHeaderRe = regexp.MustCompile(
	`^\[(\d{1,4}[./-]\d{1,2}[./-]\d{2,4}),?\s+(\d{1,2}:\d{2}(?::\d{2})?(?:\s?[APap]\.?[Mm]\.?)?)\]\s*(.*)$`)

var androidHeaderRe = regexp.MustCompile(
	`^(\d{1,4}[./-]\d{1,2}[./-]\d{2,4}),?\s+(\d{1,2}:\d{2}(?::\d{2})?(?:\s?[APap]\.?[Mm]\.?)?)\s+-\s+(.*)$`)

var dateComponentsRe = regexp.MustCompile(`^(\d{1,4})[./-](\d{1,2})[./-](\d{2,4})$`)

var timeRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?\s?([APap]\.?[Mm]\.?)?$`)

// bodyRe splits "sender: message" from a header line already stripped of date/time.
var bodyRe = regexp.MustCompile(`^([^:\n]{1,100}): ([\s\S]*)$`)

// pt entries are additive only: the destructive strictGroupNoticePatterns list stays
// pt-BR-only until there is a real export to check the exact wording against. The
// nl/es/it/fr/de entries are best effort, for the same reason.

var iosAttachmentRe = regexp.MustCompile(`^<(?:anexado|attached|adjunto|allegato|joint|angehängt|bijgevoegd): (.+)>$`)
var androidAttachmentRe = regexp.MustCompile(`^(.+?) \((?:arquivo anexado|file attached|archivo adjunto|file allegato|fichier joint|Datei angehängt|bestand bijgevoegd)\)$`)

func compileList(phrases ...string) []*regexp.Regexp {
	list := make([]*regexp.Regexp, len(phrases))
	for i, phrase := range phrases {
		list[i] = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(phrase))
	}
	return list
}

// compileList's counterpart for patterns already written as an expression, without QuoteMeta.
func compileRegex(exprs ...string) []*regexp.Regexp {
	list := make([]*regexp.Regexp, len(exprs))
	for i, expr := range exprs {
		list[i] = regexp.MustCompile(`(?i)` + expr)
	}
	return list
}

// Hidden/omitted media notices.
var omittedMediaPatterns = compileList(
	"<Mídia oculta>", "<Media omitted>", "<Multimedia omitido>",
	"imagem oculta", "áudio ocultado", "figurinha não incluída", "autocolante não incluído",
	"vídeo omitido", "documento omitido", "GIF omitido",
	"imagen omitida", "audio omitido", "sticker omitido", "video omitido",
	"<Media omessi>", "immagine omessa", "audio omesso", "adesivo omesso", "video omesso", "documento omesso",
	"<Médias omis>", "image omise", "audio omis", "autocollant omis", "vidéo omise", "document omis",
	"<Medien weggelassen>", "Bild weggelassen", "Audio weggelassen", "Sticker weggelassen", "Video weggelassen", "Dokument weggelassen",
	"<Media weggelaten>", "afbeelding weggelaten", "audio weggelaten", "sticker weggelaten", "video weggelaten", "document weggelaten", "GIF weggelaten",
)

// Deleted-message variants.
var deletedPatterns = compileList(
	"Esta mensagem foi apagada", "Esta mensagem foi eliminada", "This message was deleted", "Se eliminó este mensaje",
	"Você apagou esta mensagem", "Você eliminou esta mensagem", "You deleted this message", "Eliminaste este mensaje",
	"Questo messaggio è stato eliminato", "Hai eliminato questo messaggio",
	"Ce message a été supprimé", "Vous avez supprimé ce message",
	"Diese Nachricht wurde gelöscht", "Du hast diese Nachricht gelöscht",
	"Dit bericht is verwijderd", "Je hebt dit bericht verwijderd",
)

// System notices that can arrive with a sender attached, and get reclassified.
var generalSystemPatterns = append(
	compileList(
		"criptografia de ponta a ponta", "encriptação de ponta a ponta", "end-to-end encrypt", "cifrado de extremo a extremo",
		"mensagens desta conversa são temporárias", "disappearing messages", "mensajes temporales",
		"crittografia end-to-end", "messaggi effimeri",
		"chiffrement de bout en bout", "messages éphémères",
		"Ende-zu-Ende-Verschlüsselung", "Verschwindende Nachrichten",
		"end-to-end-encryptie", "verdwijnende berichten",
	),
	// Anchored, not substring: "security code" is a phrase people write.
	compileRegex(
		`^seu código de segurança com .{1,80} mudou\.?$`,
		`^o teu código de segurança com .{1,80} mudou\.?$`,
		`^your security code with .{1,80} changed\.?$`,
		`^tu código de seguridad con .{1,80} cambió\.?$`,
		`^il tuo codice di sicurezza con .{1,80} è cambiato\.?$`,
		`^(ton|votre) code de sécurité avec .{1,80} a changé\.?$`,
		`^dein sicherheitscode (mit|für) .{1,80} hat sich geändert\.?$`,
		`^je beveiligingscode met .{1,80} is gewijzigd\.?$`,
	)...,
)

// groupSystemPatterns infer is_group. Only checked against messages that already have
// no sender, so they can be loose.
var groupSystemPatterns = compileList(
	"criou o grupo", "created group", "creó el grupo", "ha creato il gruppo", "a créé le groupe", "hat die Gruppe erstellt",
	"mudou o assunto do grupo", "changed the subject", "cambió el asunto del grupo",
	"ha cambiato l'oggetto del gruppo", "a modifié le sujet du groupe", "hat den Betreff der Gruppe geändert",
	"entrou no grupo", "joined using", "se unió usando", "usando il link", "a rejoint via", "ist über den Einladungslink beigetreten",
	"adicionou", "added", "agregó", "añadió", "ha aggiunto", "a ajouté", "hat hinzugefügt",
	"saiu do grupo", "left", "salió del grupo", "ha lasciato il gruppo", "a quitté le groupe", "hat die Gruppe verlassen",
	"removeu", "removed", "eliminó a", "quitó a", "ha rimosso", "a retiré", "hat entfernt",
	"heeft de groep aangemaakt", "heeft het onderwerp van de groep gewijzigd",
	"is toegetreden via de uitnodigingslink", "toegevoegd", "heeft de groep verlaten", "verwijderd",
)

// Group notices emitted with the group name in place of the sender. Anchored, and separate
// from groupSystemPatterns: this one erases a sender, so a loose match would eat a message.
var strictGroupNoticePatterns = compileRegex(
	`^.{1,80} (adicionou|removeu) você$`,
	`^você (adicionou|removeu) .{1,80}$`,
	`^(você|.{1,80}) saiu( do grupo)?$`,
	`^você foi (adicionad[oa]|removid[oa])( por .{1,80})?$`,
	`^.{1,80} criou (o|este) grupo.*$`,
	`^.{1,80} mudou (o assunto|a descrição|a imagem|o ícone|as configurações) (do|deste|de) grupo.*$`,
	`^.{1,80} entrou (no grupo )?usando.*link de convite.*$`,
	`^.{1,80} (agora é|não é mais) (um |uma )?administrador.*$`,
	`^.{1,80} (added|removed) you$`,
	`^you (added|removed) .{1,80}$`,
	`^(you|.{1,80}) left( the group)?$`,
	`^you were (added|removed)( by .{1,80})?$`,
	`^.{1,80} created (this |the )?group.*$`,
	`^.{1,80} changed (the subject|this group's|the group).*$`,
	`^.{1,80} joined using (this group's )?invite link.*$`,
	`^.{1,80} is (now|no longer) an admin.*$`,
	`^.{1,80} te (agregó|añadió)$`,
	`^.{1,80} te (eliminó|quitó)$`,
	`^(tú |usted )?(agregaste|añadiste) a .{1,80}$`,
	`^(tú |usted )?(eliminaste|quitaste) a .{1,80}$`,
	`^(tú|usted|.{1,80}) sali(ó|ste)( del grupo)?$`,
	`^te (agregaron|añadieron|eliminaron|quitaron)( .{1,80})?$`,
	`^.{1,80} creó (el|este) grupo.*$`,
	`^.{1,80} cambió (el asunto|la descripción|la imagen|el ícono|la configuración) (del|de este|de) grupo.*$`,
	`^.{1,80} se unió (al grupo )?usando.*enlace de invitación.*$`,
	`^.{1,80} (ahora es|ya no es) (un |una )?administrador.*$`,
	`^.{1,80} ti ha (aggiunto|rimosso)$`,
	`^hai (aggiunto|rimosso) .{1,80}$`,
	`^hai lasciato( il gruppo)?$`,
	`^.{1,80} ha lasciato( il gruppo)?$`,
	`^sei stat[oa] (aggiunt[oa]|rimoss[oa])( da .{1,80})?$`,
	`^.{1,80} ha creato (il|questo) gruppo.*$`,
	`^.{1,80} ha cambiato (l'oggetto|la descrizione|l'immagine|l'icona|le impostazioni) (del|di questo) gruppo.*$`,
	`^.{1,80} è entrat[oa] (nel gruppo )?usando.*link di invito.*$`,
	`^.{1,80} (è ora|non è più) (un.{0,2})?amministratore.*$`,
	`^.{1,80} t'a (ajouté|retiré)$`,
	`^tu as (ajouté|retiré) .{1,80}$`,
	`^tu as quitté( le groupe)?$`,
	`^.{1,80} a quitté( le groupe)?$`,
	`^tu as été (ajouté|retiré)e?( par .{1,80})?$`,
	`^.{1,80} a créé (ce |le )?groupe.*$`,
	`^.{1,80} a modifié (le sujet|la description|l'image|l'icône|les paramètres) (du|de ce) groupe.*$`,
	`^.{1,80} a rejoint (le groupe )?(via|en utilisant).*lien d'invitation.*$`,
	`^.{1,80} (est|n'est plus) administrateur.*$`,
	`^.{1,80} hat dich (hinzugefügt|entfernt)$`,
	`^du hast .{1,80} (hinzugefügt|entfernt)$`,
	`^du hast die Gruppe verlassen$`,
	`^.{1,80} hat die Gruppe verlassen$`,
	`^du wurdest (hinzugefügt|entfernt)( von .{1,80})?$`,
	`^.{1,80} hat (die|diese) Gruppe erstellt.*$`,
	`^.{1,80} hat (den Betreff|die Beschreibung|das Bild|das Symbol|die Einstellungen) (der|dieser) Gruppe geändert.*$`,
	`^.{1,80} ist über den Einladungslink beigetreten.*$`,
	`^.{1,80} ist (jetzt|kein) Administrator.*$`,
	`^.{1,80} heeft (jou|je) (toegevoegd|verwijderd)$`,
	`^je hebt .{1,80} (toegevoegd|verwijderd)$`,
	`^je hebt de groep verlaten$`,
	`^.{1,80} heeft de groep verlaten$`,
	`^je bent (toegevoegd|verwijderd)( door .{1,80})?$`,
	`^.{1,80} heeft (de|deze) groep (aangemaakt|gemaakt).*$`,
	`^.{1,80} heeft (het onderwerp|de beschrijving|de afbeelding|het pictogram|de instellingen) van (de|deze) groep gewijzigd.*$`,
	`^.{1,80} is (lid geworden|toegetreden) (via|met).*uitnodigingslink.*$`,
	`^.{1,80} is nu (beheerder|admin).*$`,
	`^.{1,80} is geen (beheerder|admin) meer.*$`,
)

// Missed-call notices.
var callPatterns = compileList(
	"chamada de voz perdida", "chamada de vídeo perdida", "chamada perdida",
	"missed voice call", "missed video call", "missed call",
	"llamada de voz perdida", "videollamada perdida", "llamada perdida",
	"chiamata vocale persa", "videochiamata persa", "chiamata persa",
	"appel vocal manqué", "appel vidéo manqué", "appel manqué",
	"verpasster Sprachanruf", "verpasster Videoanruf", "verpasster Anruf",
	"gemiste spraakoproep", "gemiste video-oproep", "gemiste oproep",
)

// Shared location.
var locationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^location:`),
	regexp.MustCompile(`(?i)^localização:`),
	regexp.MustCompile(`(?i)^ubicación:`),
	regexp.MustCompile(`(?i)^posizione:`),
	regexp.MustCompile(`(?i)^position\s?:`),
	regexp.MustCompile(`(?i)^standort:`),
	regexp.MustCompile(`(?i)^locatie:`),
	regexp.MustCompile(`(?i)live location`),
	regexp.MustCompile(`(?i)localização em tempo real`),
	regexp.MustCompile(`(?i)ubicación en tiempo real`),
	regexp.MustCompile(`(?i)posizione in tempo reale`),
	regexp.MustCompile(`(?i)position en direct`),
	regexp.MustCompile(`(?i)livestandort`),
	regexp.MustCompile(`(?i)live locatie`),
	regexp.MustCompile(`(?i)maps\.google\.com`),
}

// The prefixes WhatsApp uses to name the export, stripped when deriving the chat name.
var fileNamePrefixes = []string{
	"Conversa do WhatsApp com ",
	"Chat do WhatsApp com ",
	"WhatsApp Chat with ",
	"WhatsApp Chat - ",
	"Chat de WhatsApp con ",
	"Chat di WhatsApp con ",
	"Discussion WhatsApp avec ",
	"WhatsApp-Chat mit ",
	"WhatsApp-chat met ",
}

func matchesAny(patterns []*regexp.Regexp, text string) bool {
	for _, p := range patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

func extractAttachment(body string) (name string, ok bool) {
	if m := iosAttachmentRe.FindStringSubmatch(body); m != nil {
		return m[1], true
	}
	if m := androidAttachmentRe.FindStringSubmatch(body); m != nil {
		return m[1], true
	}
	return "", false
}

// media_kind from the file name's prefix/infix (Android/iOS) and, failing that, extension.
func classifyMedia(name string) string {
	upper := strings.ToUpper(name)
	switch {
	case strings.HasPrefix(upper, "IMG-"):
		return "image"
	case strings.HasPrefix(upper, "VID-"):
		return "video"
	case strings.HasPrefix(upper, "AUD-"):
		return "audio"
	case strings.HasPrefix(upper, "PTT-"):
		return "voice"
	case strings.HasPrefix(upper, "STK-"):
		return "sticker"
	case strings.Contains(upper, "-PHOTO-"):
		return "image"
	case strings.Contains(upper, "-VIDEO-"):
		return "video"
	case strings.Contains(upper, "-AUDIO-"):
		return "audio"
	case strings.Contains(upper, "-STICKER-"):
		return "sticker"
	case strings.Contains(upper, "-GIF-"):
		return "gif"
	case strings.Contains(upper, "-DOCUMENT-"):
		return "document"
	}

	// Media that came from outside WhatsApp — forwarded from a computer, or an archive
	// rebuilt by another tool — keeps its own name, and the prefixes above never match. The
	// extension is all that is left, and without it a photo renders as a document card.
	switch strings.ToLower(filepath.Ext(name)) {
	case ".opus":
		return "voice"
	case ".vcf":
		return "contact"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".heic", ".heif":
		return "image"
	case ".webp":
		return "sticker"
	case ".mp4", ".mov", ".3gp", ".mkv", ".avi", ".webm":
		return "video"
	case ".mp3", ".m4a", ".aac", ".ogg", ".wav", ".amr":
		return "audio"
	default:
		return "document"
	}
}

// The Unicode dashes WhatsApp uses in place of an ASCII hyphen in phone numbers.
var hyphenVariants = strings.NewReplacer(
	"‐", "-", // hyphen
	"‑", "-", // non-breaking hyphen
	"‒", "-", // figure dash
	"–", "-", // en dash
	"—", "-", // em dash
	"−", "-", // minus sign
)

// SameName compares two names ignoring case, surrounding whitespace and Unicode hyphens.
func SameName(a, b string) bool {
	return strings.EqualFold(normalizeName(a), normalizeName(b))
}

func normalizeName(s string) string {
	return hyphenVariants.Replace(strings.TrimSpace(s))
}
