// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"

	"github.com/bborbe/errors"
	core "github.com/bborbe/notification"
	"github.com/bborbe/notification/telegram"
)

// ResolveTelegramDestination returns the chat and the bot a notification is
// delivered to. An empty chat means the type is deliberately not routed to
// Telegram and the caller skips it; an unlisted type is an error.
//
// The chat and the bot are separate axes and are resolved independently:
// TelegramChatRouting owns the chat, TelegramBotRouting owns the sender, and a
// chat id cannot identify a bot. The Target-override rule below is the one
// shared decision between them, so it lives here rather than in each caller —
// the handler and the throttle cannot drift on it.
func ResolveTelegramDestination(
	ctx context.Context,
	routing TelegramChatRouting,
	botRouting TelegramBotRouting,
	notification core.Notification,
) (telegram.ChatID, telegram.Bot, error) {
	chatID, err := routing.Resolve(ctx, notification.Type)
	if err != nil {
		return "", "", errors.Wrapf(ctx, err, "resolve chat failed")
	}
	bot, err := botRouting.Resolve(ctx, notification.Type)
	if err != nil {
		return "", "", errors.Wrapf(ctx, err, "resolve bot failed")
	}
	if notification.Target != nil {
		// Target overrides the chat only. The bot stays whatever the type
		// routed to: Target names a destination within a channel ("telegram
		// room", "discord channelName"), not a sender, so letting it change the
		// bot would hand a producer the power to move an escalation onto the bot
		// that must keep alerting.
		chatID = telegram.ChatID(*notification.Target)
	}
	return chatID, bot, nil
}
