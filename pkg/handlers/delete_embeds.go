package handlers

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (h *Handler) HandleDeleteEmbeds(data discord.MessageCommandInteractionData, event *handler.CommandEvent) error {
	message := data.TargetMessage()
	messageRef := message.MessageReference
	messageCreate := discord.NewMessageCreate().WithEphemeral(true)
	if messageRef == nil || messageRef.MessageID == nil {
		return event.CreateMessage(messageCreate.WithContent("Message is not a reply."))
	}
	if message.Author.ID != h.Config.DeArrowUserID {
		return event.CreateMessage(messageCreate.WithContent("Message is not a DeArrow reply."))
	}
	rest := event.Client().Rest
	parentID := *messageRef.MessageID
	parent, err := rest.GetMessage(event.Channel().ID(), parentID)
	if err != nil {
		return event.CreateMessage(messageCreate.WithContent("Failed to fetch the parent message."))
	}
	if parent.Author.ID != event.User().ID {
		return event.CreateMessage(messageCreate.WithContent("Only the message author can delete DeArrow embeds."))
	}
	if err := event.CreateMessage(messageCreate.WithContent("Deleting DeArrow embeds.")); err != nil {
		return err
	}
	delete(h.Bot.ReplyMap, parentID) // remove parent from the map as the DeArrow reply is now gone
	return rest.DeleteMessage(event.Channel().ID(), message.ID)
}
