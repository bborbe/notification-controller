// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"

	"github.com/bborbe/errors"
	core "github.com/bborbe/notification"
	"github.com/bborbe/notification/discord"
)

// DiscordChannelRouting maps every known notification type to the Discord
// channel it is delivered to. The table is explicit by design: every member
// of core.AvailableNotificationTypes must be listed, and a type missing from
// the table fails resolution instead of silently falling through to a
// default channel.
type DiscordChannelRouting map[core.NotificationType]discord.ChannelName

// NewDiscordChannelRouting builds the routing table from the configured
// channel names. Values may repeat — most types share the default channel
// and the test type routes to the test channel; what matters is that every
// notification type is listed explicitly.
func NewDiscordChannelRouting(
	defaultChannelName discord.ChannelName,
	testChannelName discord.ChannelName,
) DiscordChannelRouting {
	return DiscordChannelRouting{
		core.AccountHitLossLimitNotificationType:   defaultChannelName,
		core.AccountHitProfitLimitNotificationType: defaultChannelName,
		core.AgentEscalationNotificationType:       defaultChannelName,
		core.BacktestCompletedNotificationType:     defaultChannelName,
		core.BacktestFailedNotificationType:        defaultChannelName,
		core.BacktestStartedNotificationType:       defaultChannelName,
		core.GchatRelevantNotificationType:         defaultChannelName,
		core.GoReleaseNotificationType:             defaultChannelName,
		core.MantraNotificationType:                defaultChannelName,
		core.PendingApprovalNotificationType:       defaultChannelName,
		core.SignalNotificationType:                defaultChannelName,
		core.TestNotificationType:                  testChannelName,
	}
}

// Resolve returns the channel routed for the notification type. An unlisted
// type is an error — never a silent fallback.
func (d DiscordChannelRouting) Resolve(
	ctx context.Context,
	notificationType core.NotificationType,
) (discord.ChannelName, error) {
	channelName, ok := d[notificationType]
	if !ok {
		return "", errors.Errorf(
			ctx,
			"no channel routed for notificationType(%s)",
			notificationType,
		)
	}
	return channelName, nil
}
