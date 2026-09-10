// Copyright (c) 2024 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
	"context"
	"net/http"
	"time"

	"github.com/bborbe/cqrs/base"
	"github.com/bborbe/cqrs/cdb"
	cqrsiam "github.com/bborbe/cqrs/iam"
	libhttp "github.com/bborbe/http"
	libkafka "github.com/bborbe/kafka"
	libkv "github.com/bborbe/kv"
	"github.com/bborbe/log"
	"github.com/bborbe/run"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/notification-controller/pkg"
	"github.com/bborbe/notification-controller/pkg/command"
	"github.com/bborbe/notification-controller/pkg/handler"
	discordcommand "github.com/bborbe/notification/command/discord"
	"github.com/bborbe/notification/command/notification"
	"github.com/bborbe/notification"
	"github.com/bborbe/notification/discord"
)

func CreateSendHandler(
	ctx context.Context,
	syncProducer libkafka.SyncProducer,
	branch base.Branch,
	initiator cqrsiam.Initiator,
) http.Handler {
	return libhttp.NewErrorHandler(
		handler.NewSendHandler(
			notification.NewNotificationPublishCommandSender(
				base.NewCommandCreator(base.RequestIDChannel(ctx)),
				cdb.NewCommandObjectSender(
					syncProducer,
					base.TopicPrefixFromBranch(branch),
					log.DefaultSamplerFactory,
				),
				initiator,
			),
		),
	)
}

func CreateNotificationConsumer(
	saramaClientProvider libkafka.SaramaClientProvider,
	syncProducer libkafka.SyncProducer,
	db libkv.DB,
	branch base.Branch,
	batchSize libkafka.BatchSize,
	initiator cqrsiam.Initiator,
	defaultChannelName discord.ChannelName,
	testChannelName discord.ChannelName,
) run.Func {
	return func(ctx context.Context) error {
		return libkafka.NewOffsetConsumerHighwaterMarksBatchWithProvider(
			saramaClientProvider,
			core.NotificationV1SchemaID.EventTopic(base.TopicPrefixFromBranch(branch)),
			libkafka.NewStoreOffsetManager(
				libkafka.NewOffsetStore(db),
				libkafka.OffsetOldest,
				libkafka.OffsetNewest,
			),
			libkafka.NewMessageHandlerBatchTxUpdate(
				db,
				libkafka.NewMessageHandlerBatchTx(
					libkafka.NewMessageHandlerTxSkipErrors(
						libkafka.NewMessageHandlerTxMetrics(
							core.NewNotificationMessageHandlerTx(
								CreateNotificationHandler(
									ctx,
									syncProducer,
									branch,
									initiator,
									defaultChannelName,
									testChannelName,
								),
							),
							libkafka.NewMetrics(),
						),
						log.DefaultSamplerFactory,
					),
				),
			),
			batchSize,
			run.NewTrigger(),
			log.DefaultSamplerFactory,
		).Consume(ctx)
	}
}

func CreateNotificationHandler(
	ctx context.Context,
	syncProducer libkafka.SyncProducer,
	branch base.Branch,
	initiator cqrsiam.Initiator,
	defaultChannelName discord.ChannelName,
	testChannelName discord.ChannelName,
) core.NotificationHandlerTx {
	return core.NotificationHandlerTxList{
		// TODO: add telegram, mail and fax :)
		CreateDiscordNotificationHandler(
			ctx,
			syncProducer,
			branch,
			initiator,
			defaultChannelName,
			testChannelName,
		),
	}
}

func CreateDiscordNotificationHandler(
	ctx context.Context,
	syncProducer libkafka.SyncProducer,
	branch base.Branch,
	initiator cqrsiam.Initiator,
	defaultChannelName discord.ChannelName,
	testChannelName discord.ChannelName,
) core.NotificationHandlerTx {
	return pkg.NewDiscordNotificationHandler(
		discordcommand.NewSendCommandObjectSender(
			base.NewCommandCreator(
				base.RequestIDChannel(ctx),
			),
			cdb.NewCommandObjectSender(
				syncProducer,
				base.TopicPrefixFromBranch(branch),
				log.DefaultSamplerFactory,
			),
			initiator,
		),
		defaultChannelName,
		testChannelName,
	)
}

func CreateCommandConsumer(
	currentTimeGetter libtime.CurrentTimeGetter,
	saramaClientProvider libkafka.SaramaClientProvider,
	syncProducer libkafka.SyncProducer,
	db libkv.DB,
	branch base.Branch,
	batchSize libkafka.BatchSize,
) run.Func {
	commandExpireDuration := 24 * time.Hour
	trigger := run.NewTrigger()
	return cdb.RunCommandConsumerTx(
		saramaClientProvider,
		syncProducer,
		db,
		core.NotificationV1SchemaID,
		batchSize,
		base.TopicPrefixFromBranch(branch),
		false,
		commandExpireDuration,
		trigger,
		cdb.CommandObjectExecutorTxs{
			command.NewNotificationPublishCommandObjectExecutor(
				currentTimeGetter,
				base.NewIdentifierGeneratorUUID[base.Identifier](),
				CreateNotificationSender(syncProducer, branch),
			),
		},
	)
}

func CreateNotificationSender(
	syncProducer libkafka.SyncProducer,
	branch base.Branch,
) pkg.NotificationSender {
	return pkg.NewNotificationSender(
		libkafka.NewJsonSender(syncProducer, log.DefaultSamplerFactory),
		branch,
	)
}
