class Message {
  final String id;
  final String channelId;
  final String authorId;
  final String content;
  final DateTime createdAt;

  const Message({
    required this.id,
    required this.channelId,
    required this.authorId,
    required this.content,
    required this.createdAt,
  });

  factory Message.fromJson(Map<String, dynamic> json) => Message(
        id: json['id'] as String,
        channelId: json['channel_id'] as String,
        authorId: json['author_id'] as String,
        content: json['content'] as String,
        createdAt: DateTime.parse(json['created_at'] as String),
      );
}
