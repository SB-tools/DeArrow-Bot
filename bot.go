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
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

const (
	dearrowApiURL          = "https://sponsor.ajay.app/api/branding?videoID=%s"
	dearrowThumbnailApiURL = "https://dearrow-thumb.ajay.app/api/v1/getThumbnail?videoID=%s"
)

func main() {
	log.SetLevel(log.LevelInfo)
	log.Info("starting the bot...")
	log.Info("disgo version: ", disgo.Version)

	client, err := disgo.New(os.Getenv("DEARROW_THUMBNAILS_TOKEN"),
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages, gateway.IntentMessageContent),
			gateway.WithPresenceOpts(gateway.WithWatchingActivity("thumbnails"))),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagsNone)),
		bot.WithEventListeners(&events.ListenerAdapter{
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

	log.Info("dearrow thumbnails bot is now running.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-s
}

func replaceYouTubeEmbed(event *events.GenericGuildMessage) {
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
	rs, err := http.Get(fmt.Sprintf(dearrowApiURL, videoID))
	if err != nil {
		log.Error("there was an error while running a branding request: ", err)
		return
	}
	if rs.StatusCode == http.StatusNotFound {
		return
	}
	defer rs.Body.Close()
	var brandingResponse BrandingResponse
	if err = json.NewDecoder(rs.Body).Decode(&brandingResponse); err != nil {
		log.Error("there was an error while decoding a branding response: ", err)
		return
	}
	author := embed.Author
	titles := brandingResponse.Titles
	thumbnails := brandingResponse.Thumbnails
	embedBuilder := discord.NewEmbedBuilder()
	embedBuilder.SetColor(embed.Color)
	embedBuilder.SetAuthor(author.Name, author.URL, author.IconURL)
	embedBuilder.SetTitle(ternary(len(titles) == 0, embed.Title, titles[0].Title))
	embedBuilder.SetURL(embed.URL)
	embedBuilder.SetImage(ternary(len(thumbnails) == 0, embed.Thumbnail.URL, fmt.Sprintf(dearrowThumbnailApiURL, videoID)))
	if len(titles) != 0 {
		embedBuilder.SetFooterText("Original title: " + embed.Title)
	}
	rest := event.Client().Rest()
	channelID := event.ChannelID
	messageID := event.MessageID
	_, err = rest.CreateMessage(channelID, discord.NewMessageCreateBuilder().
		SetEmbeds(embedBuilder.Build()).
		SetMessageReferenceByID(messageID).
		SetAllowedMentions(&discord.AllowedMentions{}).
		Build())
	if err != nil {
		log.Error("there was an error while creating a message: ", err)
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
	} `json:"thumbnails"`
}

func ternary[T any](exp bool, ifCond T, elseCond T) T {
	if exp {
		return ifCond
	}
	return elseCond
}
