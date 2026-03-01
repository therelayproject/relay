import 'package:dio/dio.dart';
import 'models/message.dart';

class MessagesClient {
  final Dio _dio;

  const MessagesClient({required Dio dio}) : _dio = dio;

  /// Sends a message to a channel.
  Future<Message> send({
    required String channelId,
    required String content,
    Map<String, dynamic>? blocks,
  }) async {
    final response = await _dio.post(
      '/api/v1/channels/$channelId/messages',
      data: {
        'channel_id': channelId,
        'content': content,
        if (blocks != null) 'blocks': blocks,
      },
    );
    return Message.fromJson(response.data as Map<String, dynamic>);
  }
}
