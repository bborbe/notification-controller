// Copyright (c) 2023 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/errors"
	libkafka "github.com/bborbe/kafka"
	"github.com/golang/glog"

	"github.com/bborbe/notification"
)

//counterfeiter:generate -o ../mocks/notification-sender.go --fake-name NotificationSender . NotificationSender
type NotificationSender interface {
	SendUpdate(ctx context.Context, notification core.Notification) error
	SendDelete(ctx context.Context, notificationIdentifier base.Identifier) error
}

func NotificationSenderFunc(
	sendUpdate func(ctx context.Context, notification core.Notification) error,
	sendDelete func(ctx context.Context, notificationIdentifier base.Identifier) error,
) NotificationSender {
	return &notificationSenderFunc{
		sendUpdate: sendUpdate,
		sendDelete: sendDelete,
	}
}

type notificationSenderFunc struct {
	sendUpdate func(ctx context.Context, notification core.Notification) error
	sendDelete func(ctx context.Context, notificationIdentifier base.Identifier) error
}

func (s notificationSenderFunc) SendUpdate(
	ctx context.Context,
	notification core.Notification,
) error {
	if s.sendUpdate == nil {
		return nil
	}
	return s.sendUpdate(ctx, notification)
}

func (s notificationSenderFunc) SendDelete(
	ctx context.Context,
	notificationIdentifier base.Identifier,
) error {
	if s.sendDelete == nil {
		return nil
	}
	return s.sendDelete(ctx, notificationIdentifier)
}

func NewNotificationSender(
	jsonSender libkafka.JsonSender,
	branch base.Branch,
) NotificationSender {
	topic := core.NotificationV1SchemaID.EventTopic(base.TopicPrefixFromBranch(branch))
	return NotificationSenderFunc(
		func(ctx context.Context, notification core.Notification) error {
			glog.V(4).Infof("send update notification %s started", notification.Identifier)
			if err := jsonSender.SendUpdate(ctx, topic, notification.Identifier, notification); err != nil {
				return errors.Wrapf(ctx, err, "send update failed")
			}
			glog.V(3).Infof("send update notification %s completed", notification.Identifier)
			return nil
		},
		func(ctx context.Context, notificationIdentifier base.Identifier) error {
			glog.V(4).Infof("send delete notification %s started", notificationIdentifier)
			if err := jsonSender.SendDelete(ctx, topic, notificationIdentifier); err != nil {
				return errors.Wrapf(ctx, err, "send delete failed")
			}
			glog.V(3).Infof("send delete notification %s completed", notificationIdentifier)
			return nil
		},
	)
}
