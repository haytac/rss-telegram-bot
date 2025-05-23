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
	"github.com/mmcdole/gofeed"
	"github.com/rs/zerolog/log"
	"github.com/haytac/rss-telegram-bot/internal/database"
	"github.com/haytac/rss-telegram-bot/pkg/interfaces"
)

const defaultParseMode = tgbotapi.ModeHTML

// DefaultFormatter implements the Formatter interface.
type DefaultFormatter struct{}

// NewDefaultFormatter creates a new DefaultFormatter.
func NewDefaultFormatter() *DefaultFormatter {
	return &DefaultFormatter{}
}

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
		"ItemContent": item.Content, // Will be processed for emojis
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

	// Replace emoji shortcodes using kyokomi/emoji/v2
	content = emoji.Sprint(content) // <--- USES kyokomi/emoji/v2

	if cfg.ReplaceEmojiImagesWithAlt {
		content = replaceEmojiImages(content) // Placeholder for HTML img emoji replacement
	}

	templateData["ItemContent"] = content // Update template data with processed content

	messageBody := content
	if cfg.MessageTemplate != "" {
		var err error
		messageBody, err = renderTemplate("message", cfg.MessageTemplate, templateData)
		if err != nil {
			log.Error().Err(err).Str("template_name", "message").Msg("Failed to render message template")
		}
	} else {
		var sb strings.Builder
		if finalTitle != "" {
			sb.WriteString(fmt.Sprintf("<b>%s</b>\n", html.EscapeString(finalTitle)))
		}
		sb.WriteString(messageBody) // messageBody already has emojis processed
		if item.Link != "" {
			sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">Read more</a>", item.Link))
		}
		messageBody = sb.String()
	}

	var fullMessage strings.Builder
	fullMessage.WriteString(messageBody)

	if cfg.IncludeAuthor && item.Author != nil && item.Author.Name != "" && !strings.Contains(messageBody, item.Author.Name) {
		fullMessage.WriteString(fmt.Sprintf("\n\n<i>Author: %s</i>", html.EscapeString(item.Author.Name)))
	}
	if len(cfg.Hashtags) > 0 && !strings.Contains(messageBody, strings.Join(cfg.Hashtags, " ")) {
		fullMessage.WriteString("\n\n")
		for _, tag := range cfg.Hashtags {
			cleanTag := strings.TrimPrefix(tag, "#")
			cleanTag = strings.ReplaceAll(cleanTag, " ", "_")
			if cleanTag != "" {
				fullMessage.WriteString(fmt.Sprintf("#%s ", cleanTag))
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
		telegraphURL, err := createTelegraphPost(finalTitle, finalMessage, authorNameForTelegraph)
		if err == nil {
			parts = append(parts, interfaces.FormattedMessagePart{
				Text:      fmt.Sprintf("View full post on Telegraph: %s", telegraphURL),
				ParseMode: defaultParseMode,
			})
			return parts, nil
		}
		log.Error().Err(err).Msg("Failed to create Telegraph post, will try splitting.")
	}

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