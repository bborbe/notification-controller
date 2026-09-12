// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"

	core "github.com/bborbe/notification"
	"github.com/bborbe/notification-controller/pkg"
	"github.com/bborbe/notification/telegram"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TelegramChatRouting", func() {
	var ctx context.Context
	var routing pkg.TelegramChatRouting
	BeforeEach(func() {
		ctx = context.Background()
		routing = pkg.NewTelegramChatRouting("112230768")
	})
	Context("Resolve", func() {
		It("lists every available notification type", func() {
			for _, notificationType := range core.AvailableNotificationTypes {
				_, err := routing.Resolve(ctx, notificationType)
				Expect(err).To(
					BeNil(),
					"notificationType(%s) missing from the routing table",
					notificationType,
				)
			}
		})
		It("lists nothing beyond the available notification types", func() {
			Expect(routing).To(HaveLen(len(core.AvailableNotificationTypes)))
		})
		It("routes agent-escalation to the chat", func() {
			chatID, err := routing.Resolve(ctx, core.AgentEscalationNotificationType)
			Expect(err).To(BeNil())
			Expect(chatID).To(Equal(telegram.ChatID("112230768")))
		})
		It("routes pending-approval to the chat", func() {
			chatID, err := routing.Resolve(ctx, core.PendingApprovalNotificationType)
			Expect(err).To(BeNil())
			Expect(chatID).To(Equal(telegram.ChatID("112230768")))
		})
		It("leaves signal unrouted", func() {
			chatID, err := routing.Resolve(ctx, core.SignalNotificationType)
			Expect(err).To(BeNil())
			Expect(chatID).To(Equal(telegram.ChatID("")))
		})
		Context("unknown type", func() {
			It("returns error", func() {
				chatID, err := routing.Resolve(ctx, core.NotificationType("unknown"))
				Expect(err).NotTo(BeNil())
				Expect(chatID).To(Equal(telegram.ChatID("")))
			})
		})
	})
})
