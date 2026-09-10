// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package handler

import (
	"context"
	"net/http"

	"github.com/bborbe/errors"
	libhttp "github.com/bborbe/http"

	"github.com/bborbe/notification/command/notification"
	"github.com/bborbe/notification"
)

func NewSendHandler(
	notificationPublishCommandSender notification.NotificationPublishCommandSender,
) libhttp.WithError {
	return libhttp.WithErrorFunc(
		func(ctx context.Context, resp http.ResponseWriter, req *http.Request) error {
			message := core.NotificationMessage(req.FormValue("message"))
			if message == "" {
				message = "test"
			}
			notificationType := core.NotificationType(req.FormValue("type"))
			if notificationType == "" {
				notificationType = core.TestNotificationType
			}
			notificationPublishCommand := notification.NotificationPublishCommand{
				Message: message,
				Type:    notificationType,
			}
			if err := notificationPublishCommandSender.SendPublishNotificationCommand(ctx, notificationPublishCommand); err != nil {
				return errors.Wrapf(ctx, err, "send command failed")
			}
			_, _ = libhttp.WriteAndGlog(resp, "send publish notification command completed")
			return nil
		},
	)
}
