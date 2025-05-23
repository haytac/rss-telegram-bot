package formatter

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
	"text/template"

	// "github.com/PuerkitoBio/goquery" // Commented out as not used yet
	"github.com/kyokomi/emoji/v2"                                // <--- CHANGED IMPORT
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
  	"github.com/microcosm-cc/bluemonday" // <--- ADD THIS IMPORT
	"github.com/mmcdole/gofeed"
	"github.com/rs/zerolog/log"
	"github.com/haytac/rss-telegram-bot/internal/database"
	"github.com/haytac/rss-telegram-bot/pkg/interfaces"
)

const defaultParseMode = tgbotapi.ModeHTML
var (
	// Define a bluemonday policy for Telegram HTML
	// This policy allows only the tags Telegram supports.
	telegramHTMLPolicy *bluemonday.Policy
)
func init() {
	telegramHTMLPolicy = bluemonday.NewPolicy()
	// Allow standard formatting tags
	telegramHTMLPolicy.AllowAttrs("href").OnElements("a")
	telegramHTMLPolicy.AllowElements("b", "strong", "i", "em", "u", "s", "strike", "del")
	telegramHTMLPolicy.AllowElements("code") // For inline code
	// For pre-formatted code blocks:
	telegramHTMLPolicy.AllowElements("pre")
	// Allow class="language-*" on <code> tags inside <pre>
	telegramHTMLPolicy.AllowAttrs("class").Matching(regexp.MustCompile("^language-[a-zA-Z0-9]+$")).OnElements("code")
	// Allow tg-spoiler tags (span or tg-spoiler element)
	telegramHTMLPolicy.AllowElements("span", "tg-spoiler")
	telegramHTMLPolicy.AllowAttrs("class").Matching(regexp.MustCompile(`^tg-spoiler$`)).OnElements("span")

	// IMPORTANT: By default, bluemonday will strip tags not explicitly allowed.
	// It will also ensure attributes are safe.
	// If you want to convert <p> to newlines, it's more complex.
	// For now, this will strip <p> tags.
}
// DefaultFormatter implements the Formatter interface.
type DefaultFormatter struct{}

// NewDefaultFormatter creates a new DefaultFormatter.

func NewDefaultFormatter() *DefaultFormatter { return &DefaultFormatter{} }

// FormatItem formats a single feed item.
func (f *DefaultFormatter) FormatItem(ctx context.Context, item *gofeed.Item, feed *database.Feed, profile *database.FormattingProfile) ([]interfaces.FormattedMessagePart, error) {
	var cfg database.FormattingProfileConfig
	if profile != nil {
		if err := profile.UnmarshalConfig(); err != nil {
			log.Warn().Err(err).Int64("profile_id", profile.ID).Msg("Failed to unmarshal formatting profile config, using defaults.")
		} else {
			cfg = profile.ParsedConfig
		}
	}

	if cfg.OmitGenericTitleRegex != "" && item.Title != "" {
		if matched, _ := regexp.MatchString(cfg.OmitGenericTitleRegex, item.Title); matched {
			log.Debug().Str("item_title", item.Title).Msg("Omitting generic item title")
			item.Title = ""
		}
	}

	var feedDisplayTitle string
	if feed.UserTitle != nil && *feed.UserTitle != "" {
		feedDisplayTitle = *feed.UserTitle
	} else {
		feedDisplayTitle = feed.URL
	}

	templateData := map[string]interface{}{
		"FeedTitle":   feedDisplayTitle,
		"FeedURL":     feed.URL,
		"ItemTitle":   item.Title,
		"ItemLink":    item.Link,
		"ItemContent": item.Content, // Raw content initially
		"ItemSummary": item.Description,
		"ItemAuthor":  "",
		"ItemDate":    item.PublishedParsed,
		"Hashtags":    strings.Join(cfg.Hashtags, " "),
	}
	if item.Author != nil {
		templateData["ItemAuthor"] = item.Author.Name
	}

	finalTitle := item.Title
	if cfg.TitleTemplate != "" {
		var err error
		finalTitle, err = renderTemplate("title", cfg.TitleTemplate, templateData)
		if err != nil {
			log.Error().Err(err).Str("template_name", "title").Msg("Failed to render title template")
		}
	}

	content := item.Content
	if content == "" {
		content = item.Description
	}

	// Process emojis first on the raw content
	contentWithEmojis := emoji.Sprint(content)

	// Sanitize the HTML content for Telegram
	// This will strip unsupported tags like <p>
	sanitizedContent := telegramHTMLPolicy.Sanitize(contentWithEmojis)

	if cfg.ReplaceEmojiImagesWithAlt {
		// This would need to run on HTML that might have img tags.
		// If emoji images are <img> tags, sanitization might remove them if not configured.
		// For now, this is a placeholder.
		sanitizedContent = replaceEmojiImages(sanitizedContent)
	}

	templateData["ItemContent"] = sanitizedContent // Use sanitized content for template

	messageBody := sanitizedContent // Start with sanitized content
	if cfg.MessageTemplate != "" {
		var err error
		// The template itself should be careful not to introduce unsupported HTML
		messageBody, err = renderTemplate("message", cfg.MessageTemplate, templateData)
		if err != nil {
			log.Error().Err(err).Str("template_name", "message").Msg("Failed to render message template")
		}
	} else {
		// Default formatting if no template
		var sb strings.Builder
		if finalTitle != "" {
			// Title is already processed by template or is raw, escape it for safety if not HTML already.
			// Assuming finalTitle is plain text here.
			sb.WriteString(fmt.Sprintf("<b>%s</b>\n", html.EscapeString(finalTitle)))
		}
		sb.WriteString(messageBody) // messageBody is already sanitized HTML
		if item.Link != "" {
			// Ensure item.Link is properly escaped if it could contain special chars, though usually URLs are fine.
			sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">Read more</a>", html.EscapeString(item.Link)))
		}
		messageBody = sb.String()
	}

	// Ensure messageBody itself is re-sanitized if the template could have introduced bad HTML.
	// However, if templates are trusted or simple, this might be overkill.
	// For safety:
	// messageBody = telegramHTMLPolicy.Sanitize(messageBody)


	var fullMessage strings.Builder
	fullMessage.WriteString(messageBody)

	if cfg.IncludeAuthor && item.Author != nil && item.Author.Name != "" && !strings.Contains(messageBody, item.Author.Name) {
		fullMessage.WriteString(fmt.Sprintf("\n\n<i>Author: %s</i>", html.EscapeString(item.Author.Name)))
	}
	if len(cfg.Hashtags) > 0 { // Simpler: just add hashtags if configured, template might handle placement
		hasHashtagsAlready := false
		for _, tag := range cfg.Hashtags {
			if strings.Contains(messageBody, "#"+strings.ReplaceAll(tag, " ", "_")) {
				hasHashtagsAlready = true
				break
			}
		}
		if !hasHashtagsAlready {
			fullMessage.WriteString("\n\n")
			for _, tag := range cfg.Hashtags {
				cleanTag := strings.TrimPrefix(tag, "#")
				cleanTag = strings.ReplaceAll(cleanTag, " ", "_")
				if cleanTag != "" {
					fullMessage.WriteString(fmt.Sprintf("#%s ", cleanTag))
				}
			}
		}
	}

	finalMessage := strings.TrimSpace(fullMessage.String())
	var parts []interfaces.FormattedMessagePart

	if cfg.UseTelegraphThresholdChars > 0 && len(finalMessage) > cfg.UseTelegraphThresholdChars {
		authorNameForTelegraph := ""
		if item.Author != nil {
			authorNameForTelegraph = item.Author.Name
		}
		// Note: finalMessage here is already HTML-sanitized for Telegram.
		// Telegraph might support more HTML. You might want to pass less sanitized content to Telegraph.
		telegraphURL, err := createTelegraphPost(finalTitle, finalMessage, authorNameForTelegraph)
		if err == nil {
			parts = append(parts, interfaces.FormattedMessagePart{
				Text:      fmt.Sprintf("View full post on Telegraph: %s", telegraphURL),
				ParseMode: defaultParseMode, // Or "" if it's just a link
			})
			return parts, nil
		}
		log.Error().Err(err).Msg("Failed to create Telegraph post, will send directly or split.")
	}

	// The finalMessage is already HTML-sanitized for Telegram.
	// The telegram.Client's SplitMessage will handle length.
	parts = append(parts, interfaces.FormattedMessagePart{Text: finalMessage, ParseMode: defaultParseMode})
	return parts, nil
}


// ... (renderTemplate, replaceEmojiImages, createTelegraphPost remain the same) ...
func renderTemplate(name, tmplStr string, data interface{}) (string, error) {
	if tmplStr == "" {
		if val, ok := data.(map[string]interface{})[name]; ok {
			if strVal, okStr := val.(string); okStr {
				return strVal, nil
			}
		}
		return "", fmt.Errorf("template string for '%s' is empty and no default value found in data", name)
	}

	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"summarize": func(s string, length int) string {
			runes := []rune(s)
			if len(runes) < length {
				return s
			}
			return string(runes[:length]) + "..."
		},
		"escapeHTML": html.EscapeString,
	}).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", name, err)
	}
	return buf.String(), nil
}

func replaceEmojiImages(htmlContent string) string {
	// Placeholder for HTML img emoji replacement logic (e.g., using goquery)
	return htmlContent
}

func createTelegraphPost(title, htmlContent, authorName string) (string, error) {
	log.Info().Str("title", title).Msg("Placeholder: Creating Telegraph post")
	return "", fmt.Errorf("telegraph posting not implemented")
}