package handlers

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (h *Handler) HandleDeleteEmbeds(event *handler.CommandEvent) error {
	data := event.MessageCommandInteractionData()
	message := data.TargetMessage()
	messageRef := message.MessageReference
	messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
	if messageRef == nil || messageRef.MessageID == nil {
		return event.CreateMessage(messageBuilder.
			SetContent("Message is not a reply.").
			Build())
	}
	if message.Author.ID != h.Config.DeArrowUserID {
		return event.CreateMessage(messageBuilder.
			SetContent("Message is not a DeArrow reply.").
			Build())
	}
	rest := event.Client().Rest()
	channelID := event.Channel().ID()
	parent, err := rest.GetMessage(channelID, *messageRef.MessageID)
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
		return err
	}
	return event.CreateMessage(messageBuilder.
		SetContent("Embeds have been deleted.").
		Build())
}
