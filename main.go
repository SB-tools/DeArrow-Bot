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
	"github.com/disgoorg/log"
	"github.com/disgoorg/snowflake/v2"
	"github.com/schollz/jsonstore"
	"golang.org/x/exp/maps"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"
)

var (
	arrowRegex = regexp.MustCompile(`(^|\s)>(\S)`)
)

const (
	dearrowThumbnailApiURL = "https://dearrow-thumb.ajay.app/api/v1/getThumbnail?videoID=%s&time=%f&generateNow=true"
)

func main() {
	log.SetLevel(log.LevelInfo)
	log.Info("starting the bot...")
	log.Info("disgo version: ", disgo.Version)

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
	}
	h := handlers.NewHandlers(b, c)

	client, err := disgo.New(os.Getenv("DEARROW_BOT_TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages, gateway.IntentMessageContent, gateway.IntentGuilds),
			gateway.WithPresenceOpts(gateway.WithWatchingActivity("YouTube embeds"))),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagChannels, cache.FlagRoles, cache.FlagMembers),
			cache.WithMemberCachePolicy(func(entity discord.Member) bool {
				return entity.User.ID == dearrowUserID
			})),
		bot.WithEventListeners(h, &events.ListenerAdapter{
			OnGuildMessageCreate: func(event *events.GuildMessageCreate) {
				replaceYouTubeEmbed(b, event.GenericGuildMessage)
			},
			OnGuildMessageUpdate: func(event *events.GuildMessageUpdate) {
				replaceYouTubeEmbed(b, event.GenericGuildMessage)
			},
		}))
	if err != nil {
		log.Fatal("error while building disgo instance: ", err)
	}

	defer client.Close(context.TODO())

	if err := client.OpenGateway(context.TODO()); err != nil {
		log.Fatal("error while connecting to the gateway: ", err)
	}

	log.Info("dearrow bot is now running.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func replaceYouTubeEmbed(bot *internal.Bot, event *events.GenericGuildMessage) {
	channel, _ := event.Channel()
	client := event.Client()
	caches := client.Caches()
	guildID := event.GuildID
	selfMember, _ := caches.SelfMember(guildID)
	permissions := caches.MemberPermissionsInChannel(channel, selfMember)
	if permissions.Missing(discord.PermissionSendMessages) {
		return
	}
	embeds := event.Message.Embeds
	if len(embeds) == 0 {
		return
	}
	videoDataMap := make(map[string]types.DeArrowEmbedData)
	rest := client.Rest()
	httpClient := rest.HTTPClient()
	thumbnailMode := bot.GetGuildData(guildID).ThumbnailMode
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
				log.Errorf("there was an error while running a branding request (%s): ", videoID, err)
				return
			}
			defer rs.Body.Close()
			var brandingResponse BrandingResponse
			if err := json.NewDecoder(rs.Body).Decode(&brandingResponse); err != nil {
				log.Errorf("there was an error while decoding a branding response (%d %s): ", rs.StatusCode, videoID, err)
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
			var replacementThumbnailURL string
			if len(thumbnails) != 0 && !thumbnails[0].Original {
				replacementThumbnailURL = formatThumbnailURL(videoID, thumbnails[0].Timestamp)
			} else {
				switch thumbnailMode {
				case types.ThumbnailModeRandomTime:
					videoDuration := brandingResponse.VideoDuration
					if videoDuration != nil && *videoDuration != 0 {
						duration := *videoDuration
						replacementThumbnailURL = formatThumbnailURL(videoID, brandingResponse.RandomTime*duration)
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
			embedData := types.DeArrowEmbedData{}
			if replacementThumbnailURL != "" {
				embedBuilder.SetImagef("attachment://thumbnail-%s.webp", videoID)
				embedData.ReplacementThumbnailURL = &replacementThumbnailURL
			}
			embedData.Embed = embedBuilder.Build()
			videoDataMap[videoID] = embedData
		}()
	}
	if len(videoDataMap) == 0 {
		return
	}
	channelID := event.ChannelID
	messageID := event.MessageID
	var dearrowEmbeds []discord.Embed
	for _, data := range maps.Values(videoDataMap) {
		dearrowEmbeds = append(dearrowEmbeds, data.Embed)
	}
	dearrowReply, err := rest.CreateMessage(channelID, discord.NewMessageCreateBuilder().
		SetEmbeds(dearrowEmbeds...).
		SetMessageReferenceByID(messageID).
		SetAllowedMentions(&discord.AllowedMentions{}).
		Build())
	if err != nil {
		log.Errorf("there was an error while creating a message in channel %d: ", channelID, err)
		return
	}
	_, err = rest.UpdateMessage(channelID, messageID, discord.NewMessageUpdateBuilder().
		SetSuppressEmbeds(true).
		Build())
	if err != nil {
		log.Error("there was an error while suppressing embeds: ", err)
		return
	}
	updateBuilder := discord.NewMessageUpdateBuilder()
	updateBuilder.SetEmbeds(dearrowEmbeds...)
	var bodies []io.ReadCloser
	for videoID, data := range videoDataMap {
		if data.ReplacementThumbnailURL == nil {
			continue
		}
		thumbnailURL := *data.ReplacementThumbnailURL
		rs, err := httpClient.Get(thumbnailURL)
		if err != nil {
			log.Errorf("there was an error while downloading a thumbnail (%s): ", thumbnailURL, err)
			continue
		}
		if rs.StatusCode != http.StatusOK {
			continue
		}
		bodies = append(bodies, rs.Body)
		updateBuilder.AddFile(fmt.Sprintf("thumbnail-%s.webp", videoID), "", rs.Body)
	}
	if len(bodies) == 0 {
		return
	}
	if _, err := rest.UpdateMessage(channelID, dearrowReply.ID, updateBuilder.Build()); err != nil {
		log.Error("there was an error while editing an embed: ", err)
	}
	for _, body := range bodies {
		body.Close()
	}
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