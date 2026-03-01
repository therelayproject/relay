/// Official Dart/Flutter SDK for the Relay API.
///
/// Usage:
/// ```dart
/// final client = RelayClient(
///   baseUrl: 'https://chat.example.com',
///   token: 'your-bot-token',
/// );
///
/// // Send a message
/// final message = await client.messages.send(
///   channelId: '1234567890',
///   content: 'Hello from a bot!',
/// );
///
/// // Stream events
/// client.events.onMessage.listen((event) {
///   print(event.message.content);
/// });
/// ```
library relay_sdk;

export 'src/client.dart';
export 'src/messages_client.dart';
export 'src/events_client.dart';
export 'src/models/message.dart';
export 'src/models/event.dart';
