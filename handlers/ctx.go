package handlers

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/log"
)

func (h *Handlers) HandleDeleteEmbeds(event *handler.CommandEvent) error {
	data := event.MessageCommandInteractionData()
	message := data.TargetMessage()
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if message.Author.ID != h.Config.DeArrowUserID {
		return event.CreateMessage(messageBuilder.
			SetContent("Message is not a DeArrow reply.").
			Build())
	}
	rest := event.Client().Rest()
	channelID := event.Channel().ID()
	parent, err := rest.GetMessage(channelID, *message.MessageReference.MessageID)
	if err != nil {
		return event.CreateMessage(messageBuilder.
			SetContent("Failed to fetch the parent message.").
			Build())
	}
	if parent.Author.ID != event.User().ID {
		return event.CreateMessage(messageBuilder.
			SetContent("Only the message author can delete DeArrow embeds.").
			Build())
	}
	if err := rest.DeleteMessage(channelID, message.ID); err != nil {
		log.Errorf("there was an error while deleting message %d in channel %d: ", message.ID, channelID, err)
		return nil
	}
	return event.CreateMessage(messageBuilder.
		SetContent("Embeds have been deleted.").
		Build())
}
