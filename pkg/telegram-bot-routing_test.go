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

var _ = Describe("TelegramBotRouting", func() {
	var ctx context.Context
	var routing pkg.TelegramBotRouting
	BeforeEach(func() {
		ctx = context.Background()
		routing = pkg.NewTelegramBotRouting("noise")
	})
	Context("Resolve", func() {
		It("lists every available notification type", func() {
			for _, notificationType := range core.AvailableNotificationTypes {
				_, err := routing.Resolve(ctx, notificationType)
				Expect(err).To(
					BeNil(),
					"notificationType(%s) missing from the bot routing table",
					notificationType,
				)
			}
		})
		It("lists nothing beyond the available notification types", func() {
			Expect(routing).To(HaveLen(len(core.AvailableNotificationTypes)))
		})
		It("returns an error for an unknown type", func() {
			_, err := routing.Resolve(ctx, core.NotificationType("nope"))
			Expect(err).NotTo(BeNil())
		})
		It("routes agent-escalation to the noise bot", func() {
			bot, err := routing.Resolve(ctx, core.AgentEscalationNotificationType)
			Expect(err).To(BeNil())
			Expect(bot).To(Equal(telegram.Bot("noise")))
		})
		// The three human-decision types must never reach the mutable bot —
		// that is the whole point of the split, so each is asserted by name
		// rather than as a group.
		It("keeps pending-approval on the default bot", func() {
			bot, err := routing.Resolve(ctx, core.PendingApprovalNotificationType)
			Expect(err).To(BeNil())
			Expect(bot).To(Equal(telegram.Bot("")))
		})
		It("keeps account-hit-loss-limit on the default bot", func() {
			bot, err := routing.Resolve(ctx, core.AccountHitLossLimitNotificationType)
			Expect(err).To(BeNil())
			Expect(bot).To(Equal(telegram.Bot("")))
		})
		It("keeps account-hit-profit-limit on the default bot", func() {
			bot, err := routing.Resolve(ctx, core.AccountHitProfitLimitNotificationType)
			Expect(err).To(BeNil())
			Expect(bot).To(Equal(telegram.Bot("")))
		})
		It("routes exactly one type to the noise bot", func() {
			noise := 0
			for _, notificationType := range core.AvailableNotificationTypes {
				bot, err := routing.Resolve(ctx, notificationType)
				Expect(err).To(BeNil())
				if bot == "noise" {
					noise++
				}
			}
			Expect(noise).To(Equal(1))
		})
	})
})
