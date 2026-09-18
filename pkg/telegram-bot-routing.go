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

// TelegramBotRouting maps every known notification type to the Telegram bot
// that delivers it. It exists because a chat id cannot identify a bot: for a
// private chat the chat id is the recipient's own user id, so every bot
// messaging the same person reports the same chat. Which bot sends a message is
// therefore a separate axis from which chat it goes to, and this table owns it.
//
// The split is by urgency, not by producer: machine-generated noise goes to a
// bot the operator can mute, and everything needing a human decision stays on
// the bot that must keep alerting. The table is explicit by design — every
// member of core.AvailableNotificationTypes is listed, so a new notification
// type fails the unit test instead of silently arriving on the muted bot.
type TelegramBotRouting map[core.NotificationType]telegram.Bot

// NewTelegramBotRouting builds the routing table from the configured noise bot.
//
// Only agent-escalation is machine noise today. Every other type resolves to
// the empty bot, which is the original bot — the one the operator keeps
// unmuted. Types that reach no chat at all still appear here with the empty
// bot; the chat routing decides whether they are sent, this decides who sends
// them.
func NewTelegramBotRouting(noiseBot telegram.Bot) TelegramBotRouting {
	return TelegramBotRouting{
		core.AccountHitLossLimitNotificationType:   "",
		core.AccountHitProfitLimitNotificationType: "",
		core.AgentEscalationNotificationType:       noiseBot,
		core.BacktestCompletedNotificationType:     "",
		core.BacktestFailedNotificationType:        "",
		core.BacktestStartedNotificationType:       "",
		core.GchatRelevantNotificationType:         "",
		core.GoReleaseNotificationType:             "",
		core.MantraNotificationType:                "",
		core.PendingApprovalNotificationType:       "",
		core.SignalNotificationType:                "",
		core.TestNotificationType:                  "",
	}
}

// Resolve returns the bot routed for the notification type. An unlisted type is
// an error — never a silent fallback to the empty bot, which would deliver an
// unknown type on the unmuted bot without anyone choosing that.
func (t TelegramBotRouting) Resolve(
	ctx context.Context,
	notificationType core.NotificationType,
) (telegram.Bot, error) {
	bot, ok := t[notificationType]
	if !ok {
		return "", errors.Errorf(
			ctx,
			"no bot routed for notificationType(%s)",
			notificationType,
		)
	}
	return bot, nil
}
