# sdk-dart

Dart client SDK for the Relay API. Used internally by the Flutter app and
available for third-party Dart/Flutter bot developers.

**Status:** Placeholder — implementation starts in Phase 3.

## Planned API

```dart
final client = RelayClient(
  baseUrl: 'https://chat.example.com',
  token: 'your-bot-token',
);

// Send a message
final message = await client.messages.send(
  channelId: '1234567890',
  content: 'Hello from a bot!',
);

// Stream events
client.events.onMessage.listen((event) {
  print(event.message.content);
});
```
