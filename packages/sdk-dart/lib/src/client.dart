import 'package:dio/dio.dart';
import 'messages_client.dart';
import 'events_client.dart';

/// Top-level entry point for the Relay SDK.
class RelayClient {
  final String baseUrl;
  final String token;
  late final Dio _dio;

  late final MessagesClient messages;
  late final EventsClient events;

  RelayClient({
    required this.baseUrl,
    required this.token,
  }) {
    _dio = Dio(BaseOptions(
      baseUrl: baseUrl,
      headers: {'Authorization': 'Bearer $token'},
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 30),
    ));

    messages = MessagesClient(dio: _dio);
    events = EventsClient(baseUrl: baseUrl, token: token);
  }

  void close() {
    events.close();
    _dio.close();
  }
}
