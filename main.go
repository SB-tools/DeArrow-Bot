package main

import (
	"context"
	"dearrow-thumbnails/handlers"
	"dearrow-thumbnails/internal"
	"dearrow-thumbnails/types"
	"dearrow-thumbnails/util"
	"fmt"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/json"
	"github.com/disgoorg/snowflake/v2"
	"github.com/getsentry/sentry-go"
	"github.com/lmittmann/tint"
	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry"
	"github.com/schollz/jsonstore"
	"golang.org/x/exp/maps"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

var (
	arrowRegex  = regexp.MustCompile(`(^|\s)>(\S)`)
	priorityKey = os.Getenv("DEARROW_PRIORITY_KEY")
	environment = os.Getenv("DEARROW_ENVIRONMENT")
	replyMap    = make(map[snowflake.ID]snowflake.ID)
)

const (
	dearrowThumbnailApiURL = "https://dearrow-thumb.ajay.app/api/v1/getThumbnail?videoID=%s&time=%f&generateNow=true"
)

func main() {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:           os.Getenv("SENTRY_DSN"),
		EnableTracing: false,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if environment == "PROD" { // only log events in prod
				return event
			}
			return nil
		},
	})
	if err != nil {
		panic(err)
	}

	defer sentry.Flush(2 * time.Second)

	logger := slog.New(slogmulti.Fanout(
		tint.NewHandler(os.Stdout, &tint.Options{
			Level: slog.LevelInfo,
		}),
		slogsentry.Option{Level: slog.LevelWarn}.NewSentryHandler()))
	slog.SetDefault(logger)

	slog.Info("starting the bot...", slog.String("disgo.version", disgo.Version))

	storagePath := os.Getenv("DEARROW_STORAGE_PATH")
	k, err := jsonstore.Open(storagePath)
	if err != nil {
		panic(err)
	}

	dearrowUserID := snowflake.GetEnv("DEARROW_USER_ID")
	c := &internal.Config{
		StoragePath:   storagePath,
		DeArrowUserID: dearrowUserID,
	}

	b := &internal.Bot{
		Keystore: k,
		Client:   &http.Client{Timeout: 5 * time.Second},
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
			OnGuildMessageCreate: func(event *events.GuildMessageCreate) {
				replaceYouTubeEmbeds(b, event.GenericGuildMessage)
			},
			OnGuildMessageUpdate: func(event *events.GuildMessageUpdate) {
				replaceYouTubeEmbeds(b, event.GenericGuildMessage)
			},
			OnGuildMessageDelete: func(event *events.GuildMessageDelete) {
				messageID := event.MessageID
				if replyID, ok := replyMap[messageID]; ok {
					rest := event.Client().Rest()
					if err := rest.DeleteMessage(event.ChannelID, replyID); err != nil {
						slog.Error("there was an error while deleting a reply", slog.Any("reply.id", replyID), tint.Err(err))
					}
					delete(replyMap, messageID)
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

	startCleanScheduler()

	slog.Info("dearrow bot is now running.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func replaceYouTubeEmbeds(bot *internal.Bot, event *events.GenericGuildMessage) {
	message := event.Message
	embeds := message.Embeds
	if len(embeds) == 0 {
		return
	}
	originalMessageID := message.ID
	if _, ok := replyMap[originalMessageID]; ok || message.Author.Bot {
		return
	}
	channel, ok := event.Channel()
	if !ok {
		slog.Warn("channel missing in cache", slog.Any("channel.id", event.ChannelID))
		return
	}
	client := event.Client()
	caches := client.Caches()
	selfMember, _ := caches.SelfMember(event.GuildID)
	permissions := caches.MemberPermissionsInChannel(channel, selfMember)
	if permissions.Missing(discord.PermissionSendMessages, discord.PermissionManageMessages, discord.PermissionEmbedLinks) {
		return
	}
	videoDataMap := make(map[string]types.DeArrowEmbedData)
	rest := client.Rest()
	httpClient := rest.HTTPClient()
	thumbnailMode := bot.GetGuildData(event.GuildID).ThumbnailMode
	for _, embed := range embeds {
		provider := embed.Provider
		if provider == nil || provider.Name != "YouTube" {
			continue
		}
		u, _ := url.Parse(embed.URL)
		videoID := u.Query().Get("v")
		if videoID == "" {
			continue
		}
		if _, ok := videoDataMap[videoID]; ok {
			continue
		}
		func() {
			rs, err := util.FetchVideoBranding(httpClient, videoID, false)
			if err != nil {
				slog.Error("there was an error while running a branding request", slog.String("video.id", videoID), tint.Err(err))
				return
			}
			status := rs.StatusCode
			if status != http.StatusOK && status != http.StatusNotFound {
				slog.Warn("received an unexpected code from a branding response", slog.Int("status.code", status), slog.String("video.id", videoID))
				return
			}
			defer rs.Body.Close()
			var brandingResponse BrandingResponse
			if err := json.NewDecoder(rs.Body).Decode(&brandingResponse); err != nil {
				slog.Error("there was an error while decoding a branding response", slog.Int("status.code", status), slog.String("video.id", videoID), tint.Err(err))
				return
			}
			titles := brandingResponse.Titles
			thumbnails := brandingResponse.Thumbnails
			embedBuilder := discord.EmbedBuilder{Embed: embed}
			embedBuilder.SetImage(embed.Thumbnail.URL)
			embedBuilder.SetThumbnail("")
			embedBuilder.SetDescription("")
			replaceTitle := len(titles) != 0 && !titles[0].Original && titles[0].Votes > -1
			if replaceTitle {
				embedBuilder.SetFooterText("Original title: " + embed.Title)
				embedBuilder.SetTitle(arrowRegex.ReplaceAllString(titles[0].Title, "$1$2"))
			}
			embedData := types.DeArrowEmbedData{}
			if len(thumbnails) != 0 {
				if !thumbnails[0].Original {
					embedData.ReplacementThumbnailURL = formatThumbnailURL(videoID, thumbnails[0].Timestamp)
				}
			} else {
				switch thumbnailMode {
				case types.ThumbnailModeRandomTime:
					videoDuration := brandingResponse.VideoDuration
					if videoDuration != nil && *videoDuration != 0 {
						duration := *videoDuration
						embedData.ReplacementThumbnailURL = formatThumbnailURL(videoID, brandingResponse.RandomTime*duration)
					} else if !replaceTitle {
						return
					}
				case types.ThumbnailModeBlank:
					embedBuilder.SetImage("")
				case types.ThumbnailModeOriginal:
					if !replaceTitle {
						return
					}
				}
			}
			if embedData.ReplacementThumbnailURL != "" {
				embedBuilder.SetImagef("attachment://thumbnail-%s.webp", videoID)
			}
			embedData.Embed = embedBuilder.Build()
			videoDataMap[videoID] = embedData
		}()
	}
	if len(videoDataMap) == 0 {
		return
	}
	channelID := event.ChannelID
	var dearrowEmbeds []discord.Embed
	for _, data := range maps.Values(videoDataMap) {
		dearrowEmbeds = append(dearrowEmbeds, data.Embed)
	}
	dearrowReply, err := rest.CreateMessage(channelID, discord.NewMessageCreateBuilder().
		SetEmbeds(dearrowEmbeds...).
		SetMessageReferenceByID(originalMessageID).
		SetAllowedMentions(&discord.AllowedMentions{}).
		Build())
	if err != nil {
		slog.Error("there was an error while creating a message in channel", slog.Any("channel.id", channelID), tint.Err(err))
		return
	}
	replyMap[originalMessageID] = dearrowReply.ID
	_, err = rest.UpdateMessage(channelID, originalMessageID, discord.NewMessageUpdateBuilder().
		SetSuppressEmbeds(true).
		Build())
	if err != nil {
		slog.Error("there was an error while suppressing embeds", tint.Err(err))
		return
	}
	updateBuilder := discord.NewMessageUpdateBuilder()
	updateBuilder.SetEmbeds(dearrowEmbeds...)
	var bodies []io.ReadCloser
	for videoID, data := range videoDataMap {
		thumbnailURL := data.ReplacementThumbnailURL
		if thumbnailURL == "" {
			continue
		}
		req, err := http.NewRequest(http.MethodGet, thumbnailURL, nil)
		if err != nil {
			slog.Error("there was an error while creating a request for a thumbnail", slog.String("thumbnail.url", thumbnailURL), tint.Err(err))
			return
		}
		req.Header.Add("Authorization", priorityKey)
		rs, err := httpClient.Do(req)
		if err != nil {
			slog.Error("there was an error while downloading a thumbnail", slog.String("thumbnail.url", thumbnailURL), tint.Err(err))
			continue
		}
		if rs.StatusCode != http.StatusOK {
			slog.Warn("received an unexpected code from a thumbnail response", slog.Int("status.code", rs.StatusCode), slog.String("video.id", videoID), slog.String("thumbnail.url", thumbnailURL))
			continue
		}
		bodies = append(bodies, rs.Body)
		updateBuilder.AddFile(fmt.Sprintf("thumbnail-%s.webp", videoID), "", rs.Body)
	}
	if len(bodies) == 0 {
		return
	}
	if _, err := rest.UpdateMessage(channelID, dearrowReply.ID, updateBuilder.Build()); err != nil {
		slog.Error("there was an error while editing an embed", slog.Any("channel.id", channelID), tint.Err(err))
	}
	for _, body := range bodies {
		body.Close()
	}
}

func startCleanScheduler() {
	time.AfterFunc(24*time.Hour, func() {
		clear(replyMap)
		startCleanScheduler()
	})
}

type BrandingResponse struct {
	Titles []struct {
		Title    string `json:"title"`
		Original bool   `json:"original"`
		Votes    int    `json:"votes"`
	} `json:"titles"`
	Thumbnails []struct {
		Timestamp float64 `json:"timestamp"`
		Original  bool    `json:"original"`
	} `json:"thumbnails"`
	RandomTime    float64  `json:"randomTime"`
	VideoDuration *float64 `json:"videoDuration"`
}

func formatThumbnailURL(videoID string, timestamp float64) string {
	return fmt.Sprintf(dearrowThumbnailApiURL, videoID, timestamp)
}
