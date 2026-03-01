# sdk-go

Go client SDK for the Relay API. Intended for bot developers and integration authors.

**Status:** Placeholder — implementation starts in Phase 3.

## Planned API

```go
client := relay.NewClient("https://chat.example.com", relay.WithToken("your-bot-token"))

// Send a message
msg, err := client.Messages.Send(ctx, relay.SendMessageInput{
    ChannelID: "1234567890",
    Content:   "Hello from a bot!",
})

// Listen to events
client.Events.On(relay.EventMessage, func(e relay.Event) {
    fmt.Println(e.Message.Content)
})
```
