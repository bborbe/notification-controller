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

// TelegramChatRouting maps every known notification type to the Telegram chat
// it is delivered to, or to the empty chat when the type is not routed to
// Telegram at all. The table is explicit by design: every member of
// core.AvailableNotificationTypes is listed, so a new notification type fails
// the unit test instead of silently reaching (or silently missing) the phone.
//
// Telegram is the phone channel and carries only the high-tier types:
// pending-approval, agent-escalation and the two account-limit types.
// Everything else — signal in particular, which belongs on Discord — resolves
// to the empty chat and is skipped by the handler.
//
// This table decides WHICH CHAT a type reaches, never which bot sends it. A
// chat id cannot identify a bot — for a private chat it is the recipient's own
// user id, so every bot serving one person shares it, and all the entries below
// are the same chat. TelegramBotRouting owns the sender.
type TelegramChatRouting map[core.NotificationType]telegram.ChatID

// NewTelegramChatRouting builds the routing table from the configured chat.
// The two high-tier types route to it; every other type is listed explicitly
// as unrouted, which the handler treats as a skip rather than an error.
func NewTelegramChatRouting(chatID telegram.ChatID) TelegramChatRouting {
	return TelegramChatRouting{
		core.AccountHitLossLimitNotificationType:   chatID,
		core.AccountHitProfitLimitNotificationType: chatID,
		core.AgentEscalationNotificationType:       chatID,
		core.BacktestCompletedNotificationType:     "",
		core.BacktestFailedNotificationType:        "",
		core.BacktestStartedNotificationType:       "",
		core.GchatRelevantNotificationType:         "",
		core.GoReleaseNotificationType:             "",
		core.MantraNotificationType:                "",
		core.PendingApprovalNotificationType:       chatID,
		core.SignalNotificationType:                "",
		core.TestNotificationType:                  "",
	}
}

// Resolve returns the chat routed for the notification type. An unlisted type
// is an error — never a silent fallback. A listed type with an empty chat is
// deliberately unrouted and the handler skips it.
func (t TelegramChatRouting) Resolve(
	ctx context.Context,
	notificationType core.NotificationType,
) (telegram.ChatID, error) {
	chatID, ok := t[notificationType]
	if !ok {
		return "", errors.Errorf(
			ctx,
			"no chat routed for notificationType(%s)",
			notificationType,
		)
	}
	return chatID, nil
}
