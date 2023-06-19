package main

import (
	"context"
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
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	ks            *jsonstore.JSONStore
	storagePath   = os.Getenv("DEARROW_STORAGE_PATH")
	dearrowUserID = snowflake.GetEnv("DEARROW_USER_ID")
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
	message := event.Message
	embeds := message.Embeds
	if len(embeds) == 0 {
		return
	}
	var embed discord.Embed
	for _, e := range embeds {
		provider := e.Provider
		if provider != nil && provider.Name == "YouTube" {
			embed = e
			break
		}
		return
	}
	u, _ := url.Parse(embed.URL)
	videoID := u.Query().Get("v")
	if videoID == "" {
		return
	}
	path := fmt.Sprintf(dearrowApiURL, videoID)
	rs, err := http.Get(path)
	if err != nil {
		log.Errorf("there was an error while running a branding request (%s): ", path, err)
		return
	}
	defer rs.Body.Close()
	var brandingResponse BrandingResponse
	if err = json.NewDecoder(rs.Body).Decode(&brandingResponse); err != nil {
		log.Errorf("there was an error while decoding a branding response (%d %s): ", rs.StatusCode, path, err)
		return
	}
	author := embed.Author
	titles := brandingResponse.Titles
	thumbnails := brandingResponse.Thumbnails
	title := embed.Title
	thumbnailURL := embed.Thumbnail.URL
	embedBuilder := discord.NewEmbedBuilder()
	embedBuilder.SetColor(embed.Color)
	embedBuilder.SetAuthor(author.Name, author.URL, author.IconURL)
	embedBuilder.SetURL(embed.URL)
	if len(titles) != 0 {
		title = strings.ReplaceAll(titles[0].Title, ">", "")
		embedBuilder.SetFooterText("Original title: " + embed.Title)
	}
	embedBuilder.SetTitle(title)

	if len(thumbnails) != 0 && !thumbnails[0].Original {
		thumbnailURL = formatThumbnailURL(videoID, thumbnails[0].Timestamp)
	} else {
		thumbnailMode := getGuildData(guildID).ThumbnailMode
		switch thumbnailMode {
		case ThumbnailModeRandomTime:
			videoDuration := brandingResponse.VideoDuration
			if videoDuration != nil && *videoDuration != 0 {
				duration := *videoDuration
				thumbnailURL = formatThumbnailURL(videoID, brandingResponse.RandomTime*duration)
			}
		case ThumbnailModeBlank:
			thumbnailURL = ""
		case ThumbnailModeOriginal:
			if len(titles) == 0 {
				return
			}
		}
	}
	embedBuilder.SetImage(thumbnailURL)

	rest := client.Rest()
	channelID := event.ChannelID
	messageID := event.MessageID
	_, err = rest.CreateMessage(channelID, discord.NewMessageCreateBuilder().
		SetEmbeds(embedBuilder.Build()).
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
	}
}

type BrandingResponse struct {
	Titles []struct {
		Title string `json:"title"`
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

func getGuildData(guildID snowflake.ID) (guildData GuildData) {
	if err := ks.Get(guildID.String(), &guildData); err != nil {
		if _, ok := err.(jsonstore.NoSuchKeyError); !ok {
			log.Errorf("there was an error while getting data for guild %d: ", guildID, err)
		}
	}
	return
}

func formatThumbnailURL(videoID string, timestamp float64) string {
	return fmt.Sprintf(dearrowThumbnailApiURL, videoID, timestamp)
}
