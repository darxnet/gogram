package gogram

import "context"

// EditText replaces the text of msg, a message the bot sent or may edit.
// The client's default parse mode applies unless opts set one or entities.
func (c *Client) EditText(
	ctx context.Context,
	msg *Message,
	text string,
	opts ...EditMessageTextOption,
) (*Message, error) {
	params := &EditMessageTextParams{ChatID: msg.Chat.Target(), MessageID: msg.MessageID, Text: text}
	params.Option(opts...)

	if mode := c.cfg.defaultParseMode; mode != "" && params.ParseMode == "" && len(params.Entities) == 0 {
		params.ParseMode = mode
	}

	return c.EditMessageText(ctx, params)
}

// EditRichText replaces the content of msg with rich content.
func (c *Client) EditRichText(
	ctx context.Context,
	msg *Message,
	rich *InputRichMessage,
	opts ...EditMessageTextOption,
) (*Message, error) {
	params := &EditMessageTextParams{ChatID: msg.Chat.Target(), MessageID: msg.MessageID, RichMessage: rich}
	params.Option(opts...)

	return c.EditMessageText(ctx, params)
}

// EditReplyMarkup replaces the inline keyboard of msg; nil removes it.
func (c *Client) EditReplyMarkup(ctx context.Context, msg *Message, markup *InlineKeyboardMarkup) (*Message, error) {
	return c.EditMessageReplyMarkup(ctx, &EditMessageReplyMarkupParams{
		ChatID:      msg.Chat.Target(),
		MessageID:   msg.MessageID,
		ReplyMarkup: markup,
	})
}

// Delete deletes msg.
func (c *Client) Delete(ctx context.Context, msg *Message) error {
	_, err := c.DeleteMessage(ctx, &DeleteMessageParams{ChatID: msg.Chat.Target(), MessageID: msg.MessageID})
	return err
}
