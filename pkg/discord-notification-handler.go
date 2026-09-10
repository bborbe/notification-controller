// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/errors"
	libkv "github.com/bborbe/kv"
	"github.com/golang/glog"

	command "github.com/bborbe/notification/command/discord"
	"github.com/bborbe/notification"
	"github.com/bborbe/notification/discord"
)

// NewDiscordNotificationHandler returns a notificationHandler
// that handles all notification send to discord.
func NewDiscordNotificationHandler(
	sendCommandObjectSender command.SendCommandObjectSender,
	defaultChannelName discord.ChannelName,
	testChannelName discord.ChannelName,
) core.NotificationHandlerTx {
	return core.NotificationHandlerTxFunc(
		func(ctx context.Context, tx libkv.Tx, notification core.Notification) error {
			glog.V(2).Infof("handling notification(%s) started", notification.Type)
			var channelName discord.ChannelName
			switch notification.Type {
			case core.TestNotificationType:
				channelName = testChannelName //"test"
			default:
				channelName = defaultChannelName //"notifications"
			}
			if notification.Target != nil {
				channelName = discord.ChannelName(*notification.Target)
				glog.V(3).Infof("found target => set channelName to %s", channelName)
			}
			sendCommand := command.SendCommand{
				ChannelName: channelName,
				Message:     discord.Message(notification.Message.String()),
			}
			if err := sendCommandObjectSender.SendCommand(ctx, sendCommand); err != nil {
				return errors.Wrapf(ctx, err, "send command failed")
			}
			glog.V(2).Infof("handling notification(%s) finished", notification.Type)
			return nil
		},
		func(ctx context.Context, tx libkv.Tx, identifier base.Identifier) error {
			return nil
		},
	)
}
