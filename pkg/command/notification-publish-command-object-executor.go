// Copyright (c) 2024 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package command

import (
	"context"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	"github.com/bborbe/errors"
	libkv "github.com/bborbe/kv"
	"github.com/bborbe/time"
	"github.com/golang/glog"

	"github.com/bborbe/notification-controller/pkg"
	"github.com/bborbe/notification/command/notification"
	"github.com/bborbe/notification"
)

func NewNotificationPublishCommandObjectExecutor(
	currentTimeGetter time.CurrentTimeGetter,
	identifierGenerator base.IdentifierGenerator[base.Identifier],
	notificationSender pkg.NotificationSender,
) cdb.CommandObjectExecutorTx {
	return cdb.CommandObjectExecutorTxFunc(
		notification.NotificationPublishCommandOperation,
		true,
		func(ctx context.Context, tx libkv.Tx, commandObject cdb.CommandObject) (*base.EventID, base.Event, error) {
			glog.V(3).Infof("send notification started")
			var command notification.NotificationPublishCommand
			if err := commandObject.Command.Data.MarshalInto(ctx, &command); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "marshal into notification failed")
			}
			if err := command.Validate(ctx); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "validate command failed")
			}

			now := currentTimeGetter.Now()
			obj := core.Notification{
				Object: base.Object[base.Identifier]{
					Identifier: identifierGenerator.NewIdentifier(),
					Created:    time.DateTime(now),
					Modified:   time.DateTime(now),
				},
				Message:  command.Message,
				Target:   command.Target,
				Type:     command.Type,
				Metadata: command.Metadata,
			}
			if err := notificationSender.SendUpdate(ctx, obj); err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "send failed")
			}
			event, err := base.ParseEvent(ctx, obj)
			if err != nil {
				return nil, nil, errors.Wrapf(ctx, err, "parse notification failed")
			}

			glog.V(3).Infof("send notification completed")
			return base.EventID(obj.Identifier).Ptr(), event, nil
		},
	)
}
