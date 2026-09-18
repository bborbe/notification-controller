// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"

	kvmocks "github.com/bborbe/kv/mocks"
	core "github.com/bborbe/notification"
	"github.com/bborbe/notification-controller/pkg"
	"github.com/bborbe/notification/mocks"
	"github.com/bborbe/notification/telegram"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TelegramNotificationHandler", func() {
	var ctx context.Context
	var err error
	var notificationHandler core.NotificationHandlerTx
	var commandSendCommandObjectSender *mocks.CommandTelegramSendCommandObjectSender
	BeforeEach(func() {
		ctx = context.Background()
		commandSendCommandObjectSender = &mocks.CommandTelegramSendCommandObjectSender{}
		notificationHandler = pkg.NewTelegramNotificationHandler(
			commandSendCommandObjectSender,
			pkg.NewTelegramChatRouting("112230768"),
			pkg.NewTelegramBotRouting("noise"),
		)
	})
	Context("UpdateNotification", func() {
		var notification core.Notification
		BeforeEach(func() {
			notification = core.Notification{
				Type:    core.AgentEscalationNotificationType,
				Message: "hello world",
			}
		})
		JustBeforeEach(func() {
			err = notificationHandler.UpdateNotification(ctx, &kvmocks.Tx{}, notification)
		})
		Context("routed type without target", func() {
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("sends the notification to the routed chat", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(1))
				_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(sendCommand.ChatID).To(Equal(telegram.ChatID("112230768")))
				Expect(sendCommand.Message).To(Equal(telegram.Message("hello world")))
			})
		})
		Context("unrouted type", func() {
			BeforeEach(func() {
				notification.Type = core.SignalNotificationType
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("sends nothing", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(0))
			})
		})
		Context("unrouted type with target override", func() {
			BeforeEach(func() {
				notification.Type = core.SignalNotificationType
				target := core.NotificationTarget("999")
				notification.Target = &target
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("sends to the override chat", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(1))
				_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(sendCommand.ChatID).To(Equal(telegram.ChatID("999")))
			})
		})
		Context("unknown type", func() {
			BeforeEach(func() {
				notification.Type = core.NotificationType("unknown")
			})
			It("returns error", func() {
				Expect(err).NotTo(BeNil())
			})
			It("sends nothing", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(0))
			})
		})
		Context("bot routing", func() {
			It("stamps the noise bot on an agent-escalation", func() {
				_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(sendCommand.Bot).To(Equal(telegram.Bot("noise")))
			})
			Context("pending-approval", func() {
				BeforeEach(func() {
					notification.Type = core.PendingApprovalNotificationType
				})
				It("stamps the default bot", func() {
					_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
					Expect(sendCommand.Bot).To(Equal(telegram.Bot("")))
				})
			})
			Context("account-hit-loss-limit", func() {
				BeforeEach(func() {
					notification.Type = core.AccountHitLossLimitNotificationType
				})
				It("reaches the chat at all", func() {
					Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(1))
					_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
					Expect(sendCommand.ChatID).To(Equal(telegram.ChatID("112230768")))
				})
				It("stamps the default bot", func() {
					_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
					Expect(sendCommand.Bot).To(Equal(telegram.Bot("")))
				})
			})
		})
		// Target names a destination within a channel, never a sender. The
		// override chat below is deliberately NOT 112230768: with the routed
		// chat id the override path and the default path produce an identical
		// ChatID, so the assertion would hold on a build that ignores Target
		// entirely — and the bot assertion beside it would hold on a build
		// where Target wrongly clears the bot. A distinct chat makes both
		// halves able to fail.
		Context("routed type with target override", func() {
			BeforeEach(func() {
				notification.Type = core.AgentEscalationNotificationType
				target := core.NotificationTarget("-1001234567890")
				notification.Target = &target
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("overrides the chat", func() {
				_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(sendCommand.ChatID).To(Equal(telegram.ChatID("-1001234567890")))
			})
			It("leaves the bot resolved from the type", func() {
				_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(sendCommand.Bot).To(Equal(telegram.Bot("noise")))
			})
		})
	})
})
