// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"
	"time"

	kvmocks "github.com/bborbe/kv/mocks"
	core "github.com/bborbe/notification"
	"github.com/bborbe/notification-controller/pkg"
	telegramcommand "github.com/bborbe/notification/command/telegram"
	"github.com/bborbe/notification/mocks"
	"github.com/bborbe/notification/telegram"
	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TelegramThrottle", func() {
	var (
		ctx                            context.Context
		currentTime                    libtime.CurrentTime
		commandSendCommandObjectSender *mocks.CommandTelegramSendCommandObjectSender
		telegramThrottle               *pkg.TelegramThrottle
		window                         time.Duration
		startedAt                      time.Time
		sendCommandCount               func() int
		sendCommandAt                  func(index int) telegramcommand.SendCommand
		publish                        func(notificationType core.NotificationType, message string)
	)

	BeforeEach(func() {
		ctx = context.Background()
		window = pkg.DefaultTelegramThrottleWindow
		startedAt = time.Date(2026, 9, 21, 19, 0, 0, 0, time.UTC)

		currentTime = libtime.NewCurrentTime()
		currentTime.SetNow(startedAt)

		commandSendCommandObjectSender = &mocks.CommandTelegramSendCommandObjectSender{}

		telegramChatRouting := pkg.NewTelegramChatRouting("112230768")
		telegramBotRouting := pkg.NewTelegramBotRouting("noise")
		telegramThrottle = pkg.NewTelegramThrottle(
			pkg.NewTelegramNotificationHandler(
				commandSendCommandObjectSender,
				telegramChatRouting,
				telegramBotRouting,
			),
			commandSendCommandObjectSender,
			telegramChatRouting,
			telegramBotRouting,
			window,
			currentTime,
		)

		sendCommandCount = func() int {
			return commandSendCommandObjectSender.SendCommandCallCount()
		}
		sendCommandAt = func(index int) telegramcommand.SendCommand {
			_, sendCommand := commandSendCommandObjectSender.SendCommandArgsForCall(index)
			return sendCommand
		}
		publish = func(notificationType core.NotificationType, message string) {
			Expect(telegramThrottle.UpdateNotification(ctx, &kvmocks.Tx{}, core.Notification{
				Type:    notificationType,
				Message: core.NotificationMessage(message),
			})).To(BeNil())
		}
	})

	// advance moves the injected clock and runs the flusher, mirroring what the
	// background Run loop does on its own tick.
	advance := func(duration time.Duration) {
		currentTime.SetNow(currentTime.Now().Add(duration))
		Expect(telegramThrottle.Flush(ctx)).To(BeNil())
	}

	Context("window length", func() {
		BeforeEach(func() {
			publish(core.PendingApprovalNotificationType, "A")
			publish(core.PendingApprovalNotificationType, "B")
		})
		It("sends the first notification immediately", func() {
			Expect(sendCommandCount()).To(Equal(1))
			Expect(sendCommandAt(0).Message).To(Equal(telegram.Message("A")))
		})
		It("holds the second until the window edge", func() {
			Expect(sendCommandCount()).To(Equal(1))

			advance(4*time.Minute + 59*time.Second)
			Expect(sendCommandCount()).To(Equal(1))

			advance(time.Second)
			Expect(sendCommandCount()).To(Equal(2))
		})
		It("coalesces the buffered notifications into one message", func() {
			advance(window)

			Expect(sendCommandCount()).To(Equal(2))
			Expect(sendCommandAt(1).Message).To(ContainSubstring("1 notifications"))
		})
	})

	Context("coalescing several notifications", func() {
		BeforeEach(func() {
			publish(core.PendingApprovalNotificationType, "gate one")
			publish(core.PendingApprovalNotificationType, "gate two")
			publish(core.PendingApprovalNotificationType, "gate three")
		})
		It("sends the first immediately and buffers the rest", func() {
			Expect(sendCommandCount()).To(Equal(1))
		})
		It(
			"emits exactly one coalesced message carrying every buffered payload and a count",
			func() {
				advance(window)

				Expect(sendCommandCount()).To(Equal(2))
				coalesced := sendCommandAt(1).Message.String()
				Expect(coalesced).To(ContainSubstring("2 notifications"))
				Expect(coalesced).To(ContainSubstring("gate two"))
				Expect(coalesced).To(ContainSubstring("gate three"))
				Expect(coalesced).NotTo(ContainSubstring("gate one"))
			},
		)
		It("stamps the routed chat and bot on the coalesced message", func() {
			advance(window)

			Expect(sendCommandAt(1).ChatID).To(Equal(telegram.ChatID("112230768")))
			Expect(sendCommandAt(1).Bot).To(Equal(telegram.Bot("")))
		})
	})

	Context("idle window reset", func() {
		BeforeEach(func() {
			publish(core.PendingApprovalNotificationType, "A")
		})
		// The inverse of the coalescing case. Without it a build that throttles
		// EVERYTHING — never sending a leading edge — would satisfy the
		// coalescing assertions above, so the suite could not tell "coalescing
		// works" from "everything is throttled". This case fails on exactly that
		// build, and it deliberately does NOT call the flusher: the reset must
		// come from the elapsed-time check, not from a flush tick.
		It("delivers immediately after a quiet window, without a flush tick", func() {
			Expect(sendCommandCount()).To(Equal(1))

			currentTime.SetNow(currentTime.Now().Add(5*time.Minute + time.Second))

			publish(core.PendingApprovalNotificationType, "B")

			Expect(sendCommandCount()).To(Equal(2))
			Expect(sendCommandAt(1).Message).To(Equal(telegram.Message("B")))
		})
	})

	Context("burst straddling the window edge", func() {
		// The soft limit is load-bearing: the post-flush arrival is a fresh
		// leading edge, so this third message is accepted rather than a defect.
		BeforeEach(func() {
			publish(core.PendingApprovalNotificationType, "A")
			publish(core.PendingApprovalNotificationType, "B")
			currentTime.SetNow(startedAt.Add(window))
			publish(core.PendingApprovalNotificationType, "C")
		})
		It("flushes the buffer and sends the arrival as a fresh leading edge", func() {
			Expect(sendCommandCount()).To(Equal(3))
			Expect(sendCommandAt(1).Message).To(ContainSubstring("1 notifications"))
			Expect(sendCommandAt(1).Message).To(ContainSubstring("B"))
			Expect(sendCommandAt(2).Message).To(Equal(telegram.Message("C")))
		})
		It("leaves nothing buffered for a later flush", func() {
			advance(window)
			Expect(sendCommandCount()).To(Equal(3))
		})
	})

	Context("exempt account-limit types", func() {
		BeforeEach(func() {
			publish(core.PendingApprovalNotificationType, "gate")
		})
		It("delivers account-loss-limit immediately inside an active window", func() {
			publish(core.AccountHitLossLimitNotificationType, "loss limit hit")

			Expect(sendCommandCount()).To(Equal(2))
			Expect(sendCommandAt(1).Message).To(Equal(telegram.Message("loss limit hit")))
		})
		It("delivers account-profit-limit immediately inside an active window", func() {
			publish(core.AccountHitProfitLimitNotificationType, "profit limit hit")

			Expect(sendCommandCount()).To(Equal(2))
			Expect(sendCommandAt(1).Message).To(Equal(telegram.Message("profit limit hit")))
		})
		It("never folds an exempt type into the coalesced message", func() {
			publish(core.AccountHitLossLimitNotificationType, "loss limit hit")
			publish(core.AccountHitProfitLimitNotificationType, "profit limit hit")
			advance(window)

			Expect(sendCommandCount()).To(Equal(3))
			for index := 0; index < sendCommandCount(); index++ {
				Expect(
					sendCommandAt(index).Message.String(),
				).NotTo(ContainSubstring("notifications"))
			}
		})
		It("does not advance the window, so a gate is not suppressed behind it", func() {
			publish(core.AccountHitLossLimitNotificationType, "loss limit hit")

			currentTime.SetNow(startedAt.Add(window + time.Second))
			publish(core.PendingApprovalNotificationType, "later gate")

			Expect(sendCommandCount()).To(Equal(3))
			Expect(sendCommandAt(2).Message).To(Equal(telegram.Message("later gate")))
		})
	})

	Context("per-bot lanes", func() {
		// pending-approval and agent-escalation share one chat but route to
		// different bots, so each bot's lane is rate-limited on its own and a
		// manager gate is never coalesced onto the bot that must keep alerting.
		BeforeEach(func() {
			publish(core.PendingApprovalNotificationType, "gate")
			publish(core.AgentEscalationNotificationType, "escalation")
		})
		It("gives each bot its own leading edge", func() {
			Expect(sendCommandCount()).To(Equal(2))
			Expect(sendCommandAt(0).Bot).To(Equal(telegram.Bot("")))
			Expect(sendCommandAt(1).Bot).To(Equal(telegram.Bot("noise")))
		})
		It("buffers per bot rather than per chat", func() {
			publish(core.PendingApprovalNotificationType, "gate two")
			publish(core.AgentEscalationNotificationType, "escalation two")

			Expect(sendCommandCount()).To(Equal(2))

			advance(window)

			Expect(sendCommandCount()).To(Equal(4))
			// Flush walks the window map, so the order two destinations flush in
			// is iteration order. Assert on the set of flushed messages keyed by
			// bot, never on the sequence.
			flushed := map[telegram.Bot]string{
				sendCommandAt(2).Bot: sendCommandAt(2).Message.String(),
				sendCommandAt(3).Bot: sendCommandAt(3).Message.String(),
			}
			Expect(flushed).To(HaveLen(2))
			Expect(flushed).To(HaveKey(telegram.Bot("")))
			Expect(flushed[telegram.Bot("")]).To(ContainSubstring("gate two"))
			Expect(flushed).To(HaveKey(telegram.Bot("noise")))
			Expect(flushed[telegram.Bot("noise")]).To(ContainSubstring("escalation two"))
		})
	})

	Context("unrouted type", func() {
		It("sends nothing", func() {
			publish(core.SignalNotificationType, "signal")
			Expect(sendCommandCount()).To(Equal(0))
		})
	})
})
