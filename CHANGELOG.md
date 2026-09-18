# Changelog

All notable changes to this project will be documented in this file.

Please choose versions by [Semantic Versioning](http://semver.org/).

* MAJOR version when you make incompatible API changes,
* MINOR version when you add functionality in a backwards-compatible manner, and
* PATCH version when you make backwards-compatible bug fixes.

## Unreleased

- docs: correct the notification-type strings named in the v0.6.0 entry below. It said `account-hit-loss-limit` and `account-hit-profit-limit`; those are the **Go constant identifiers** (`AccountHitLossLimitNotificationType`), not the values that travel on the wire. The wire strings are `account-loss-limit` and `account-profit-limit` — no `hit` — per the `NotificationType` const block in `github.com/bborbe/notification`, which is the only authority for them. The fix described in v0.6.0 is real and verified end-to-end on dev; only the names used to describe it were wrong. Corrected here rather than in place because a released section is immutable. Anyone reproducing the verification from the v0.6.0 wording will otherwise publish the wrong type and get `validate Type failed: notificationType(account-hit-loss-limit) is invalid`, which reads as a routing regression rather than a typo.

## v0.6.0

- feat: add `TelegramBotRouting`, mapping each notification type to the bot that delivers it, and stamp the resolved bot on the outgoing `SendCommand`. A chat id cannot identify a bot — for a private chat it is the recipient's own user id — so which bot sends a message is a separate axis from which chat it reaches.
- feat: add `TELEGRAM_NOISE_BOT`, naming the bot that carries machine noise. `agent-escalation` routes to it; every other type stays on the default bot. Empty (the default) keeps all types on the original bot, so an unconfigured deployment is unchanged.
- fix: route `account-hit-loss-limit` and `account-hit-profit-limit` to the telegram chat. Both were listed as deliberately unrouted and never reached the phone, though they are human-decision types that should. They now arrive on the default bot alongside `pending-approval`.
- docs: state in both routing tables that `TelegramChatRouting` owns the chat and `TelegramBotRouting` owns the sender, and that `notification.Target` overrides the chat only — never the bot, which would let a producer move an escalation onto the bot that must keep alerting.
- chore: bump `github.com/bborbe/notification` to v0.7.0 for the `Bot` field on `SendCommand`.

## v0.5.0

- feat: add telegram notification handler as the second entry of the handler list

## v0.4.0

- feat: explicit notification type → Discord channel routing table (replaces the switch-with-default; unlisted types now fail instead of silently falling back)

## v0.3.1

- chore: bump github.com/bborbe/notification to v0.3.0 (adds gchat-relevant notification type)

## v0.3.0

- fix: rebuild image from post-bump tree (v0.2.0 image was built from pre-bump code and lacks go-release type support)

## v0.2.0

- bump github.com/bborbe/notification to v0.2.0 (adds go-release notification type support)

## v0.1.0

- feat: extract notification controller service from trading monorepo (consumes notification topic, routes to channel handlers)
