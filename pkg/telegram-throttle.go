// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/errors"
	libkv "github.com/bborbe/kv"
	core "github.com/bborbe/notification"
	telegramcommand "github.com/bborbe/notification/command/telegram"
	"github.com/bborbe/notification/telegram"
	libtime "github.com/bborbe/time"
	"github.com/golang/glog"
)

// DefaultTelegramThrottleWindow is the channel-level rate limit applied to the
// throttled Telegram notification types.
const DefaultTelegramThrottleWindow = 5 * time.Minute

// telegramThrottleFlushInterval is how often the flusher looks for windows that
// have reached their edge. It bounds how late a coalesced message can be, so it
// stays far below the window itself.
const telegramThrottleFlushInterval = time.Second

// telegramThrottleDestination identifies the lane being rate-limited. The bot
// is part of the identity, not just the chat: pending-approval and
// agent-escalation share one chat while routing to different bots, and folding
// them into one message would have to pick a single sender — which is exactly
// the failure TelegramBotRouting exists to prevent, since an escalation must
// never move onto the bot that carries manager gates.
type telegramThrottleDestination struct {
	chatID telegram.ChatID
	bot    telegram.Bot
}

// telegramThrottleWindow is one open rate-limit window: the instant it closes
// and the notifications buffered inside it.
type telegramThrottleWindow struct {
	flushAt time.Time
	buffer  []telegram.Message
}

// TelegramThrottle applies a soft channel-level rate limit to the throttled
// Telegram notification types.
//
// The throttle wraps the notification handler rather than the command sender
// because the handler is the last point at which notification.Type is still in
// scope: a telegramcommand.SendCommand carries only ChatID/Message/Bot, so no
// type survives to the sender and the two trading-alert types could not be
// exempted at that seam.
//
// The limit is deliberately soft. A notification arriving with no open window
// is sent immediately and opens a window; further ones inside that window are
// buffered and flushed as a single coalesced message at the window edge. A
// burst that straddles the edge therefore produces one extra message, because
// the post-flush arrival is a fresh leading edge — an accepted overshoot, not a
// defect.
//
//counterfeiter:generate -o ../mocks/telegram-throttle.go --fake-name TelegramThrottle . TelegramThrottle
type TelegramThrottle interface {
	core.NotificationHandlerTx
	// Run flushes window edges until the context is cancelled.
	Run(ctx context.Context) error
	// Flush emits the coalesced message for every destination whose window has
	// reached its edge, then closes those windows.
	Flush(ctx context.Context) error
}

type telegramThrottle struct {
	inner             core.NotificationHandlerTx
	sender            telegramcommand.SendCommandObjectSender
	routing           TelegramChatRouting
	botRouting        TelegramBotRouting
	window            time.Duration
	currentTimeGetter libtime.CurrentTimeGetter

	mutex   sync.Mutex
	windows map[telegramThrottleDestination]*telegramThrottleWindow
}

// NewTelegramThrottle wraps a Telegram notification handler with a rate limit
// of the given window length. The clock is injected so callers can advance time
// rather than sleep.
func NewTelegramThrottle(
	inner core.NotificationHandlerTx,
	sender telegramcommand.SendCommandObjectSender,
	routing TelegramChatRouting,
	botRouting TelegramBotRouting,
	window time.Duration,
	currentTimeGetter libtime.CurrentTimeGetter,
) TelegramThrottle {
	return &telegramThrottle{
		inner:             inner,
		sender:            sender,
		routing:           routing,
		botRouting:        botRouting,
		window:            window,
		currentTimeGetter: currentTimeGetter,
		windows:           make(map[telegramThrottleDestination]*telegramThrottleWindow),
	}
}

// UpdateNotification implements core.NotificationHandlerTx.
//
// The two account-limit types are handed straight to the wrapped handler: they
// are never buffered and they neither open nor advance a window, so a trading
// alert can be neither delayed by this throttle nor able to suppress a gate
// queued behind it.
func (t *telegramThrottle) UpdateNotification(
	ctx context.Context,
	tx libkv.Tx,
	notification core.Notification,
) error {
	if isTelegramThrottleExempt(notification.Type) {
		return t.inner.UpdateNotification(ctx, tx, notification)
	}
	chatID, bot, err := ResolveTelegramDestination(ctx, t.routing, t.botRouting, notification)
	if err != nil {
		return errors.Wrapf(ctx, err, "resolve destination failed")
	}
	if chatID == "" {
		glog.V(2).Infof(
			"notification(%s) not routed to telegram => skipped",
			notification.Type,
		)
		return nil
	}
	return t.offer(
		ctx,
		telegramThrottleDestination{chatID: chatID, bot: bot},
		telegram.Message(notification.Message.String()),
	)
}

// DeleteNotification implements core.NotificationHandlerTx and is delegated
// unchanged; the throttle holds no state a deletion could invalidate.
func (t *telegramThrottle) DeleteNotification(
	ctx context.Context,
	tx libkv.Tx,
	identifier base.Identifier,
) error {
	return t.inner.DeleteNotification(ctx, tx, identifier)
}

// Run flushes window edges until the context is cancelled. It exists because
// the handler runs inside a CQRS notification handler and must not block the
// consumer, so the buffered batch is emitted from here rather than inline.
//
// The buffer is in memory: a pod restart loses a batch that has not yet been
// flushed. That is a named, accepted cost, not a defect.
func (t *telegramThrottle) Run(ctx context.Context) error {
	ticker := time.NewTicker(telegramThrottleFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := t.Flush(ctx); err != nil {
				return errors.Wrapf(ctx, err, "flush failed")
			}
		}
	}
}

// Flush emits the coalesced message for every destination whose window has
// reached its edge, then closes those windows. A window that reaches its edge
// with an empty buffer is closed too, so the next notification on that
// destination starts a fresh leading edge instead of waiting.
func (t *telegramThrottle) Flush(ctx context.Context) error {
	now := t.currentTimeGetter.Now()

	t.mutex.Lock()
	defer t.mutex.Unlock()

	for destination, window := range t.windows {
		if now.Before(window.flushAt) {
			continue
		}
		if len(window.buffer) > 0 {
			if err := t.send(
				ctx,
				destination,
				coalesceTelegramMessages(window.buffer, t.window),
			); err != nil {
				return errors.Wrapf(ctx, err, "send coalesced message failed")
			}
		}
		delete(t.windows, destination)
	}
	return nil
}

// offer sends the message immediately when the destination has no window or the
// window has already passed its edge by wall clock, and buffers it otherwise.
// Reading the clock rather than trusting an open window is what lets an idle
// destination reset: after a quiet period the next notification is a leading
// edge even if no flush tick has run.
//
// An arrival that finds a past-edge window still holding a buffer flushes that
// buffer first, so a buffered batch is never carried into the next window.
func (t *telegramThrottle) offer(
	ctx context.Context,
	destination telegramThrottleDestination,
	message telegram.Message,
) error {
	now := t.currentTimeGetter.Now()

	t.mutex.Lock()
	defer t.mutex.Unlock()

	window, ok := t.windows[destination]
	if ok && now.Before(window.flushAt) {
		window.buffer = append(window.buffer, message)
		glog.V(2).Infof(
			"notification throttled chat(%s) bot(%s) buffered(%d)",
			destination.chatID,
			destination.bot,
			len(window.buffer),
		)
		return nil
	}
	if !ok {
		window = &telegramThrottleWindow{}
		t.windows[destination] = window
	}
	if len(window.buffer) > 0 {
		buffered := window.buffer
		window.buffer = nil
		if err := t.send(
			ctx,
			destination,
			coalesceTelegramMessages(buffered, t.window),
		); err != nil {
			return errors.Wrapf(ctx, err, "send coalesced message failed")
		}
	}
	window.flushAt = now.Add(t.window)
	return t.send(ctx, destination, message)
}

func (t *telegramThrottle) send(
	ctx context.Context,
	destination telegramThrottleDestination,
	message telegram.Message,
) error {
	if err := t.sender.SendCommand(ctx, telegramcommand.SendCommand{
		ChatID:  destination.chatID,
		Message: message,
		Bot:     destination.bot,
	}); err != nil {
		return errors.Wrapf(ctx, err, "send command failed")
	}
	return nil
}

// coalesceTelegramMessages folds the buffered payloads into one message that
// carries every one of them verbatim plus a count, so the count and the texts
// can be checked against each other and no notification is dropped by the
// coalescing path.
func coalesceTelegramMessages(
	buffered []telegram.Message,
	window time.Duration,
) telegram.Message {
	parts := make([]string, 0, len(buffered))
	for _, message := range buffered {
		parts = append(parts, string(message))
	}
	return telegram.Message(fmt.Sprintf(
		"%d notifications in the last %s:\n\n%s",
		len(buffered),
		window,
		strings.Join(parts, "\n\n"),
	))
}

// isTelegramThrottleExempt reports whether a notification type bypasses the
// throttle entirely. The two account-limit types are trading alerts where a
// delay could cost capital, so they always send immediately.
//
// The exemption keys on the Go constants, never on the wire strings
// ("account-loss-limit"): a wire string is not a NotificationType value, so a
// comparison against it would compile and silently never fire.
func isTelegramThrottleExempt(notificationType core.NotificationType) bool {
	switch notificationType {
	case core.AccountHitLossLimitNotificationType,
		core.AccountHitProfitLimitNotificationType:
		return true
	default:
		return false
	}
}
