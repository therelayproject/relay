import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'models/event.dart';

class EventsClient {
  final String baseUrl;
  final String token;

  WebSocketChannel? _channel;
  final StreamController<RelayEvent> _controller =
      StreamController<RelayEvent>.broadcast();

  EventsClient({required this.baseUrl, required this.token});

  /// Connects to the Relay WebSocket event stream.
  void connect() {
    final wsUrl = baseUrl
        .replaceFirst('https://', 'wss://')
        .replaceFirst('http://', 'ws://');

    _channel = WebSocketChannel.connect(
      Uri.parse('$wsUrl/api/v1/ws?token=$token'),
    );

    _channel!.stream.listen(
      (data) {
        try {
          final json = jsonDecode(data as String) as Map<String, dynamic>;
          _controller.add(RelayEvent.fromJson(json));
        } catch (_) {
          // ignore malformed frames
        }
      },
      onDone: _controller.close,
    );
  }

  /// Stream of all real-time events.
  Stream<RelayEvent> get onEvent => _controller.stream;

  /// Filtered stream of message_created events.
  Stream<RelayEvent> get onMessage =>
      onEvent.where((e) => e.type == EventType.messageCreated);

  void close() {
    _channel?.sink.close();
    _controller.close();
  }
}
