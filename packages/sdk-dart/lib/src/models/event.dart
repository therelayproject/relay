import 'message.dart';

enum EventType {
  messageCreated,
  messageUpdated,
  messageDeleted,
  reactionAdded,
  presenceUpdated,
  typingStart,
  unknown,
}

EventType _parseEventType(String raw) => switch (raw) {
      'message_created' => EventType.messageCreated,
      'message_updated' => EventType.messageUpdated,
      'message_deleted' => EventType.messageDeleted,
      'reaction_added' => EventType.reactionAdded,
      'presence_updated' => EventType.presenceUpdated,
      'typing_start' => EventType.typingStart,
      _ => EventType.unknown,
    };

class RelayEvent {
  final EventType type;
  final Message? message;
  final Map<String, dynamic> raw;

  const RelayEvent({
    required this.type,
    this.message,
    required this.raw,
  });

  factory RelayEvent.fromJson(Map<String, dynamic> json) {
    final type = _parseEventType(json['type'] as String? ?? '');
    return RelayEvent(
      type: type,
      message: json['message'] != null
          ? Message.fromJson(json['message'] as Map<String, dynamic>)
          : null,
      raw: json,
    );
  }
}
