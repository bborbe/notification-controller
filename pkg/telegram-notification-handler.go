// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/errors"
	libkv "github.com/bborbe/kv"
	core "github.com/bborbe/notification"
	telegramcommand "github.com/bborbe/notification/command/telegram"
	"github.com/bborbe/notification/telegram"
	"github.com/golang/glog"
)

// NewTelegramNotificationHandler returns a notificationHandler that handles
// all notifications routed to Telegram. Types the routing table leaves
// unrouted are skipped — Telegram carries only the high-tier types, so a
// skip is the normal path for most notifications, not an error.
func NewTelegramNotificationHandler(
	sendCommandObjectSender telegramcommand.SendCommandObjectSender,
	routing TelegramChatRouting,
) core.NotificationHandlerTx {
	return core.NotificationHandlerTxFunc(
		func(ctx context.Context, tx libkv.Tx, notification core.Notification) error {
			glog.V(2).Infof("handling notification(%s) started", notification.Type)
			chatID, err := routing.Resolve(ctx, notification.Type)
			if err != nil {
				return errors.Wrapf(ctx, err, "resolve chat failed")
			}
			if notification.Target != nil {
				chatID = telegram.ChatID(*notification.Target)
				glog.V(2).Infof(
					"notification(%s) target override => chat(%s)",
					notification.Type,
					chatID,
				)
			}
			if chatID == "" {
				glog.V(2).Infof(
					"notification(%s) not routed to telegram => skipped",
					notification.Type,
				)
				return nil
			}
			glog.V(2).
				Infof("notification(%s) routed to telegram chat(%s)", notification.Type, chatID)
			sendCommand := telegramcommand.SendCommand{
				ChatID:  chatID,
				Message: telegram.Message(notification.Message.String()),
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
