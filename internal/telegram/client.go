package telegram

import (
	"context"
	"fmt"
	"sync" // Needed for Client struct's mutexes

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/haytac/rss-telegram-bot/internal/database"
	"github.com/haytac/rss-telegram-bot/pkg/interfaces" // For HTTPClientFactory and FormattedMessagePart
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate" // Needed for Client struct's limiters
)

const (
	telegramMaxMessageLength = 4096 // THIS CONSTANT MUST BE PRESENT
	globalMessagesPerSecond  = 25
	chatMessagesPerSecond    = 1
)

// Client wraps the Telegram Bot API client with rate limiting.
// THIS STRUCT DEFINITION MUST BE PRESENT
type Client struct {
	clientFactory  interfaces.HTTPClientFactory
	bots           map[string]*tgbotapi.BotAPI
	botsMu         sync.RWMutex // Uses "sync"
	globalLimiter  *rate.Limiter // Uses "golang.org/x/time/rate"
	chatLimiters   map[string]*rate.Limiter
	chatLimitersMu sync.Mutex // Uses "sync"
}

// NewClient creates a new Telegram client.
func NewClient(clientFactory interfaces.HTTPClientFactory) *Client { // Returns *Client
	return &Client{ // Uses Client
		clientFactory: clientFactory,
		bots:          make(map[string]*tgbotapi.BotAPI),
		globalLimiter: rate.NewLimiter(rate.Limit(globalMessagesPerSecond), globalMessagesPerSecond*2),
		chatLimiters:  make(map[string]*rate.Limiter),
	}
}

func (c *Client) getBotAPI(botToken string, proxy *database.Proxy) (*tgbotapi.BotAPI, error) {
	c.botsMu.RLock() // Uses c.botsMu
	bot, exists := c.bots[botToken]
	c.botsMu.RUnlock()
	if exists {
		return bot, nil
	}
	c.botsMu.Lock()
	defer c.botsMu.Unlock()
	if bot, exists = c.bots[botToken]; exists {
		return bot, nil
	}
	httpClient, err := c.clientFactory.GetClient(proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP client for Telegram bot: %w", err)
	}
	api, err := tgbotapi.NewBotAPIWithClient(botToken, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API instance: %w", err)
	}
	log.Info().Str("bot_username", api.Self.UserName).Msg("Telegram bot authorized")
	c.bots[botToken] = api
	return api, nil
}

func (c *Client) getChatLimiter(chatID string) *rate.Limiter {
	c.chatLimitersMu.Lock() // Uses c.chatLimitersMu
	defer c.chatLimitersMu.Unlock()
	limiter, exists := c.chatLimiters[chatID]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(chatMessagesPerSecond), chatMessagesPerSecond*2) // Uses rate.NewLimiter
		c.chatLimiters[chatID] = limiter
	}
	return limiter
}

func (c *Client) Send(ctx context.Context, botToken, chatIDStr string, parts []interfaces.FormattedMessagePart, proxy *database.Proxy) error {
	bot, err := c.getBotAPI(botToken, proxy)
	if err != nil {
		return fmt.Errorf("getting bot API: %w", err)
	}

	var numericChatID int64
	isChannelUsername := false
	if _, errScan := fmt.Sscan(chatIDStr, &numericChatID); errScan != nil {
		isChannelUsername = true
		log.Debug().Str("chat_id_str", chatIDStr).Msg("Chat ID is not numeric, treating as channel username.")
	}

	globalCtxLimiter := context.Background()
	operationLogger := log.With().Str("chat_id_str", chatIDStr).Str("bot_username", bot.Self.UserName).Logger()

	for i, part := range parts {
		if err := c.globalLimiter.Wait(globalCtxLimiter); err != nil { // Uses c.globalLimiter
			return fmt.Errorf("global rate limiter wait: %w", err)
		}
		chatLimiter := c.getChatLimiter(chatIDStr)
		if err := chatLimiter.Wait(globalCtxLimiter); err != nil {
			return fmt.Errorf("chat rate limiter wait for %s: %w", chatIDStr, err)
		}

		partLogger := operationLogger.With().Int("part_index", i).Logger()
		var msgConfig tgbotapi.Chattable

		if part.PhotoURL != "" {
			photoFile := tgbotapi.FileURL(part.PhotoURL)
			cfg := tgbotapi.PhotoConfig{
				BaseFile: tgbotapi.BaseFile{
					BaseChat: tgbotapi.BaseChat{
						ReplyToMessageID: 0,
					},
					File: photoFile,
				},
				Caption:   part.Text,
				ParseMode: part.ParseMode,
			}
			if isChannelUsername {
				cfg.BaseChat.ChannelUsername = chatIDStr
			} else {
				cfg.BaseChat.ChatID = numericChatID
			}
			msgConfig = cfg
			partLogger.Debug().Str("photo_url", part.PhotoURL).Msg("Preparing to send photo")

		} else if part.DocumentURL != "" {
			docFile := tgbotapi.FileURL(part.DocumentURL)
			cfg := tgbotapi.DocumentConfig{
				BaseFile: tgbotapi.BaseFile{
					BaseChat: tgbotapi.BaseChat{
						ReplyToMessageID: 0,
					},
					File: docFile,
				},
				Caption:   part.DocumentCaption,
				ParseMode: part.ParseMode,
			}
			if isChannelUsername {
				cfg.BaseChat.ChannelUsername = chatIDStr
			} else {
				cfg.BaseChat.ChatID = numericChatID
			}
			msgConfig = cfg
			partLogger.Debug().Str("document_url", part.DocumentURL).Msg("Preparing to send document")

		} else if part.Text != "" {
			cfg := tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ReplyToMessageID: 0,
				},
				Text:                  part.Text,
				ParseMode:             part.ParseMode,
				DisableWebPagePreview: false,
			}
			if isChannelUsername {
				cfg.BaseChat.ChannelUsername = chatIDStr
			} else {
				cfg.BaseChat.ChatID = numericChatID
			}
			msgConfig = cfg
			partLogger.Debug().Int("text_length", len(part.Text)).Msg("Preparing to send text message")
		} else {
			partLogger.Warn().Msg("Skipping message part: no text, photo, or document URL provided.")
			continue
		}

		if msgConfig == nil {
			partLogger.Error().Msg("Internal error: msgConfig is nil before sending, skipping part.")
			continue
		}

		if _, err := bot.Send(msgConfig); err != nil {
			partLogger.Error().Err(err).Msg("Failed to send message to Telegram")
			return fmt.Errorf("sending message part to chat '%s': %w", chatIDStr, err)
		}
		partLogger.Debug().Msg("Message part sent successfully")
	}
	return nil
}

// SplitMessage uses interfaces.FormattedMessagePart
func SplitMessage(text, parseMode string) []interfaces.FormattedMessagePart {
	// Uses telegramMaxMessageLength
	if len(text) <= telegramMaxMessageLength {
		return []interfaces.FormattedMessagePart{{Text: text, ParseMode: parseMode}}
	}
	var parts []interfaces.FormattedMessagePart
	runes := []rune(text)
	currentPartStartIndex := 0
	for i := 0; i < len(runes); {
		// Uses telegramMaxMessageLength
		end := currentPartStartIndex + telegramMaxMessageLength
		if end > len(runes) {
			end = len(runes)
		}
		actualEnd := end
		parts = append(parts, interfaces.FormattedMessagePart{Text: string(runes[currentPartStartIndex:actualEnd]), ParseMode: parseMode})
		currentPartStartIndex = actualEnd
		i = currentPartStartIndex
	}
	if len(parts) > 1 {
		log.Warn().Int("original_len_runes", len(runes)).Int("num_parts", len(parts)).Msg("Message split due to length")
	}
	return parts
}

func (c *Client) Name() string { // Uses *Client
	return "telegram"
}