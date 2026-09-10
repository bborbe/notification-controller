// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"

	"github.com/bborbe/cqrs/base"
	kvmocks "github.com/bborbe/kv/mocks"
	core "github.com/bborbe/notification"
	"github.com/bborbe/notification-controller/pkg"
	"github.com/bborbe/notification/discord"
	"github.com/bborbe/notification/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiscordNotificationHandler", func() {
	var ctx context.Context
	var err error
	var notificationHandler core.NotificationHandlerTx
	var commandSendCommandObjectSender *mocks.CommandSendCommandObjectSender
	BeforeEach(func() {
		ctx = context.Background()
		commandSendCommandObjectSender = &mocks.CommandSendCommandObjectSender{}
		notificationHandler = pkg.NewDiscordNotificationHandler(
			commandSendCommandObjectSender,
			"notifications",
			"test",
		)
	})
	Context("UpdateNotification", func() {
		var notification core.Notification
		BeforeEach(func() {
			notification = core.Notification{}
		})
		JustBeforeEach(func() {
			err = notificationHandler.UpdateNotification(ctx, &kvmocks.Tx{}, notification)
		})
		Context("test type without target", func() {
			BeforeEach(func() {
				notification.Type = core.TestNotificationType
				notification.Target = nil
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("send message", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(1))
				argCtx, argCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(argCtx).NotTo(BeNil())
				Expect(argCommand.ChannelName).To(Equal(discord.ChannelName("test")))
			})
		})
		Context("test type with target", func() {
			BeforeEach(func() {
				notification.Type = core.TestNotificationType
				notification.Target = core.NotificationTarget("banana").Ptr()
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("send message", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(1))
				argCtx, argCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(argCtx).NotTo(BeNil())
				Expect(argCommand.ChannelName).To(Equal(discord.ChannelName("banana")))
			})
		})
		Context("non-test type without target", func() {
			BeforeEach(func() {
				notification.Type = core.NotificationType("info")
				notification.Target = nil
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("send message to default channel", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(1))
				argCtx, argCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(argCtx).NotTo(BeNil())
				Expect(argCommand.ChannelName).To(Equal(discord.ChannelName("notifications")))
			})
		})
		Context("with custom message", func() {
			BeforeEach(func() {
				notification.Type = core.TestNotificationType
				notification.Message = core.NotificationMessage("Custom test message")
			})
			It("returns no error", func() {
				Expect(err).To(BeNil())
			})
			It("sends correct message content", func() {
				Expect(commandSendCommandObjectSender.SendCommandCallCount()).To(Equal(1))
				argCtx, argCommand := commandSendCommandObjectSender.SendCommandArgsForCall(0)
				Expect(argCtx).NotTo(BeNil())
				Expect(argCommand.Message).To(Equal(discord.Message("Custom test message")))
			})
		})
	})
	Context("DeleteNotification", func() {
		var identifier base.Identifier
		BeforeEach(func() {
			identifier = "1234"
		})
		JustBeforeEach(func() {
			err = notificationHandler.DeleteNotification(ctx, &kvmocks.Tx{}, identifier)
		})
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})
})
