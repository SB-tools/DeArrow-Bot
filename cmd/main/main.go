package main

import (
	"context"
	"dearrow-bot/db"
	"dearrow-bot/dearrow"
	"dearrow-bot/handlers"
	"dearrow-bot/internal"
	"dearrow-bot/util"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry"
	"golang.org/x/sync/errgroup"
)

var (
	replyMap    = make(map[snowflake.ID]snowflake.ID)
	debugLogger *slog.Logger
)

const (
	cleanPeriod = 24 * time.Hour
)

func main() {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	err = sentry.Init(sentry.ClientOptions{
		Dsn:           os.Getenv("SENTRY_DSN"),
		EnableTracing: false,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if os.Getenv("DEARROW_ENVIRONMENT") == "PROD" { // only log events in prod
				return event
			}
			return nil
		},
	})
	if err != nil {
		panic(err)
	}

	defer sentry.Flush(2 * time.Second)

	fileWriter, err := os.OpenFile("log.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer fileWriter.Close()
	debugLogger = slog.New(slog.NewTextHandler(fileWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	logger := slog.New(slogmulti.Fanout(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level: slog.LevelInfo,
		}),
		slogsentry.Option{Level: slog.LevelWarn}.NewSentryHandler()))
	slog.SetDefault(logger)

	slog.Info("starting the bot...", slog.String("disgo.version", disgo.Version))

	dearrowUserID := snowflake.GetEnv("DEARROW_USER_ID")
	c := &internal.Config{
		DeArrowUserID: dearrowUserID,
	}

	b := &internal.Bot{
		DB:       db.NewDB(pool),
		Client:   dearrow.New(util.NewBrandingClient(), util.NewThumbnailClient()),
		ReplyMap: replyMap,
	}
	h := handlers.NewHandler(b, c)

	client, err := disgo.New(os.Getenv("DEARROW_BOT_TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages, gateway.IntentMessageContent, gateway.IntentGuilds),
			gateway.WithPresenceOpts(gateway.WithWatchingActivity("YouTube embeds"))),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagChannels, cache.FlagRoles, cache.FlagMembers),
			cache.WithMemberCachePolicy(func(entity discord.Member) bool {
				return entity.User.ID == dearrowUserID
			})),
		bot.WithEventListeners(h, &events.ListenerAdapter{
			OnGuildMessageCreate: func(ev *events.GuildMessageCreate) {
				messageListener(ev.GenericGuildMessage, b)
			},
			OnGuildMessageUpdate: func(ev *events.GuildMessageUpdate) {
				if time.Since(ev.Message.ID.Time()).Hours() <= 1 { // prevent ghost edits because discord
					messageListener(ev.GenericGuildMessage, b)
				}
			},
			OnGuildMessageDelete: func(ev *events.GuildMessageDelete) {
				if replyID, ok := replyMap[ev.MessageID]; ok {
					rest := ev.Client().Rest()
					if err := rest.DeleteMessage(ev.ChannelID, replyID); err != nil {
						slog.Error("dearrow: error while deleting a reply",
							slog.Any("reply.id", replyID),
							slog.Any("parent.id", ev.MessageID),
							slog.Any("channel.id", ev.ChannelID),
							tint.Err(err))
					}
					delete(replyMap, ev.MessageID)
				}
			},
		}))
	if err != nil {
		panic(err)
	}

	defer client.Close(context.TODO())

	if err := client.OpenGateway(context.TODO()); err != nil {
		panic(err)
	}

	ticker := time.NewTicker(cleanPeriod)
	go func() {
		for {
			t := <-ticker.C
			debugLogger.Debug("dearrow: clearing reply map", slog.Time("timestamp", t), slog.Int("count", len(replyMap)))
			clear(replyMap)
		}
	}()

	slog.Info("dearrow bot is now running.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func messageListener(ev *events.GenericGuildMessage, bot *internal.Bot) {
	if len(ev.Message.Embeds) == 0 {
		return
	}
	if _, ok := replyMap[ev.MessageID]; ok || ev.Message.Author.Bot { // ignore messages which have already been replied to or bots
		return
	}
	channel, ok := ev.Channel()
	if !ok {
		slog.Warn("dearrow: channel missing in cache", slog.Any("channel.id", ev.ChannelID))
		return
	}
	client := ev.Client()
	caches := client.Caches()
	selfMember, ok := caches.SelfMember(ev.GuildID)
	if !ok {
		slog.Warn("dearrow: self member missing in cache", slog.Any("guild.id", ev.GuildID))
		return
	}
	permissions := caches.MemberPermissionsInChannel(channel, selfMember)
	debugLogger.Debug("dearrow: permissions in channel", slog.Any("channel.id", ev.ChannelID), slog.Any("permissions", permissions))

	if permissions.Missing(discord.PermissionSendMessages, discord.PermissionManageMessages, discord.PermissionEmbedLinks, discord.PermissionReadMessageHistory) {
		debugLogger.Debug("dearrow: ignoring message due to missing permissions",
			slog.Any("channel.id", ev.ChannelID),
			slog.Any("message.id", ev.MessageID),
			slog.Any("permissions", permissions))
		return
	}
	config, err := bot.DB.GetGuildConfig(ev.GuildID)
	if err != nil {
		slog.Error("dearrow: error while getting guild config", slog.Any("guild.id", ev.GuildID), tint.Err(err))
		return
	}

	replacementMap := make(map[string]*dearrow.ReplacementData)
	for _, embed := range ev.Message.Embeds {
		provider := embed.Provider
		if provider == nil || provider.Name != "YouTube" {
			continue
		}
		videoID := util.ParseVideoID(embed)
		if videoID == "" {
			continue
		}
		if _, ok := replacementMap[videoID]; ok { // ignore videos that already have a replacement
			continue
		}
		branding, err := bot.Client.FetchBranding(videoID)
		if err != nil {
			return // fail the entire process if any branding request fails for completeness
		}
		data := branding.ToReplacementData(videoID, config, embed, debugLogger)
		if data != nil {
			replacementMap[videoID] = data
		}
	}
	if len(replacementMap) == 0 { // no videos to replace, exit
		return
	}

	replyBuilder := discord.NewMessageCreateBuilder()
	replyBuilder.SetMessageReferenceByID(ev.MessageID)
	replyBuilder.SetAllowedMentions(&discord.AllowedMentions{})

	eg, ctx := errgroup.WithContext(context.Background())
	c := make(chan io.ReadCloser, len(replacementMap))
loop:
	for videoID, data := range replacementMap {
		select {
		case <-ctx.Done():
			break loop
		default:

		}

		replyBuilder.AddEmbeds(data.ToEmbed())

		timestamp := data.Timestamp
		if timestamp == -1 { // no need to fetch a new thumbnail
			continue
		}
		eg.Go(func() error {
			thumbnail, err := bot.Client.FetchThumbnail(videoID, timestamp)
			if err != nil {
				return err
			}
			c <- thumbnail
			replyBuilder.AddFile("thumbnail-"+videoID+".webp", "", thumbnail)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return
	}
	close(c)

	reply, err := client.Rest().CreateMessage(ev.ChannelID, replyBuilder.Build())

	for closer := range c {
		closer.Close()
	}

	if err != nil {
		slog.Error("dearrow: error while sending reply", slog.Any("channel.id", ev.ChannelID), slog.Any("parent.id", ev.MessageID), tint.Err(err))
		return
	}
	replyMap[ev.MessageID] = reply.ID

	if _, err := client.Rest().UpdateMessage(ev.ChannelID, ev.MessageID, discord.MessageUpdate{
		Flags: json.Ptr(ev.Message.Flags.Add(discord.MessageFlagSuppressEmbeds)), // add the bit to current flags not to override them
	}); err != nil {
		slog.Error("dearrow: error while suppressing embeds", slog.Any("channel.id", ev.ChannelID), slog.Any("message.id", ev.MessageID), tint.Err(err))
	}
}
