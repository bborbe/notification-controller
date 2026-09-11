// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"

	core "github.com/bborbe/notification"
	"github.com/bborbe/notification-controller/pkg"
	"github.com/bborbe/notification/discord"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiscordChannelRouting", func() {
	var ctx context.Context
	var routing pkg.DiscordChannelRouting
	BeforeEach(func() {
		ctx = context.Background()
		routing = pkg.NewDiscordChannelRouting("notifications", "test")
	})
	It("routes every available notification type to a channel", func() {
		for _, notificationType := range core.AvailableNotificationTypes {
			channelName, err := routing.Resolve(ctx, notificationType)
			Expect(err).To(BeNil())
			Expect(channelName).NotTo(BeEmpty())
		}
	})
	It("lists exactly the available notification types", func() {
		Expect(routing).To(HaveLen(len(core.AvailableNotificationTypes)))
	})
	It("routes the test type to the test channel", func() {
		channelName, err := routing.Resolve(ctx, core.TestNotificationType)
		Expect(err).To(BeNil())
		Expect(channelName).To(Equal(discord.ChannelName("test")))
	})
	It("routes signal and agent-escalation to the shared default channel", func() {
		signalChannelName, err := routing.Resolve(ctx, core.SignalNotificationType)
		Expect(err).To(BeNil())
		Expect(signalChannelName).To(Equal(discord.ChannelName("notifications")))
		agentEscalationChannelName, err := routing.Resolve(
			ctx,
			core.AgentEscalationNotificationType,
		)
		Expect(err).To(BeNil())
		Expect(agentEscalationChannelName).To(Equal(discord.ChannelName("notifications")))
	})
	It("fails for a type missing from the table", func() {
		channelName, err := routing.Resolve(ctx, core.NotificationType("unknown"))
		Expect(err).NotTo(BeNil())
		Expect(channelName).To(BeEmpty())
	})
})
