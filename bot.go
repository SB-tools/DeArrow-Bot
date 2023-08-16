package main

import (
	"context"
	"errors"
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
	ks            *jsonstore.JSONStore
	storagePath   = os.Getenv("DEARROW_STORAGE_PATH")
	dearrowUserID = snowflake.GetEnv("DEARROW_USER_ID")
	arrowRegex    = regexp.MustCompile(`(^|\s)>(\S)`)
)

const (
	dearrowApiURL          = "https://sponsor.ajay.app/api/branding?videoID=%s"
	dearrowThumbnailApiURL = "https://dearrow-thumb.ajay.app/api/v1/getThumbnail?videoID=%s&time=%f&generateNow=true"
)

func main() {
	log.SetLevel(log.LevelInfo)
	log.Info("starting the bot...")
	log.Info("disgo version: ", disgo.Version)

	k, err := jsonstore.Open(storagePath)
	if err != nil {
		panic(err)
	}
	ks = k

	client, err := disgo.New(os.Getenv("DEARROW_BOT_TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages, gateway.IntentMessageContent, gateway.IntentGuilds),
			gateway.WithPresenceOpts(gateway.WithWatchingActivity("YouTube embeds"))),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagChannels, cache.FlagRoles, cache.FlagMembers),
			cache.WithMemberCachePolicy(func(entity discord.Member) bool {
				return entity.User.ID == dearrowUserID
			})),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnApplicationCommandInteraction: onCommand,
			OnGuildMessageCreate: func(event *events.GuildMessageCreate) {
				replaceYouTubeEmbed(event.GenericGuildMessage)
			},
			OnGuildMessageUpdate: func(event *events.GuildMessageUpdate) {
				replaceYouTubeEmbed(event.GenericGuildMessage)
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

func onCommand(event *events.ApplicationCommandInteractionCreate) {
	data := event.SlashCommandInteractionData()
	guildID := event.GuildID()
	messageBuilder := discord.NewMessageCreateBuilder()
	switch *data.SubCommandName {
	case "get":
		messageBuilder.SetContentf("Current mode is set to **%s**.", getGuildData(*guildID).ThumbnailMode)
	case "set":
		thumbnailMode := ThumbnailMode(data.Int("mode"))
		err := ks.Set(guildID.String(), GuildData{
			ThumbnailMode: thumbnailMode,
		})
		if err != nil {
			log.Errorf("there was an error while setting mode %d for guild %d: ", thumbnailMode, guildID, err)
			return
		}
		if err := jsonstore.Save(ks, storagePath); err != nil {
			log.Errorf("there was an error while saving data for guild %d: ", guildID, err)
			return
		}
		messageBuilder.SetContentf("Mode has been set to **%s**.", thumbnailMode)
	}
	err := event.CreateMessage(messageBuilder.
		SetEphemeral(true).
		Build())
	if err != nil {
		log.Error("there was an error while creating a command response: ", err)
	}
}

func replaceYouTubeEmbed(event *events.GenericGuildMessage) {
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
	videoDataMap := make(map[string]DeArrowEmbedData)
	rest := client.Rest()
	httpClient := rest.HTTPClient()
	thumbnailMode := getGuildData(guildID).ThumbnailMode
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
			path := fmt.Sprintf(dearrowApiURL, videoID)
			rs, err := httpClient.Get(path)
			if err != nil {
				log.Errorf("there was an error while running a branding request (%s): ", path, err)
				return
			}
			defer rs.Body.Close()
			var brandingResponse BrandingResponse
			if err := json.NewDecoder(rs.Body).Decode(&brandingResponse); err != nil {
				log.Errorf("there was an error while decoding a branding response (%d %s): ", rs.StatusCode, path, err)
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
				case ThumbnailModeRandomTime:
					videoDuration := brandingResponse.VideoDuration
					if videoDuration != nil && *videoDuration != 0 {
						duration := *videoDuration
						replacementThumbnailURL = formatThumbnailURL(videoID, brandingResponse.RandomTime*duration)
					} else if !replaceTitle {
						return
					}
				case ThumbnailModeBlank:
					embedBuilder.SetImage("")
				case ThumbnailModeOriginal:
					if !replaceTitle {
						return
					}
				}
			}
			embedData := DeArrowEmbedData{}
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

type GuildData struct {
	ThumbnailMode ThumbnailMode `json:"thumbnail_mode"`
}

type ThumbnailMode int

const (
	ThumbnailModeRandomTime ThumbnailMode = iota
	ThumbnailModeBlank
	ThumbnailModeOriginal
)

func (t ThumbnailMode) String() string {
	switch t {
	case ThumbnailModeRandomTime:
		return "Show a screenshot from a random time"
	case ThumbnailModeBlank:
		return "Show no thumbnail"
	case ThumbnailModeOriginal:
		return "Show the original thumbnail"
	}
	return "Unknown"
}

type DeArrowEmbedData struct {
	Embed                   discord.Embed
	ReplacementThumbnailURL *string
}

func getGuildData(guildID snowflake.ID) (guildData GuildData) {
	if err := ks.Get(guildID.String(), &guildData); err != nil {
		var noSuchKeyError jsonstore.NoSuchKeyError
		if !errors.As(err, &noSuchKeyError) {
			log.Errorf("there was an error while getting data for guild %d: ", guildID, err)
		}
	}
	return
}

func formatThumbnailURL(videoID string, timestamp float64) string {
	return fmt.Sprintf(dearrowThumbnailApiURL, videoID, timestamp)
}
