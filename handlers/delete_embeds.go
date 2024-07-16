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
	parentID := *messageRef.MessageID
	parent, err := rest.GetMessage(event.Channel().ID(), parentID)
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
	if err := event.CreateMessage(messageBuilder.
		SetContent("Deleting DeArrow embeds.").
		Build()); err != nil {
		return err
	}
	delete(h.Bot.ReplyMap, parentID) // remove parent from the map as the DeArrow reply is now gone
	return rest.DeleteMessage(event.Channel().ID(), message.ID)
}
