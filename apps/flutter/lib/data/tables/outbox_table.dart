import 'package:drift/drift.dart';

/// Outbox queue for messages composed while offline.
/// The OutboxWorker replays these on WebSocket reconnect.
class OutboxTable extends Table {
  @override
  String get tableName => 'outbox';

  IntColumn get id => integer().autoIncrement()();
  TextColumn get channelId => text()();
  TextColumn get content => text()();
  TextColumn get blocks => text().nullable()();

  /// ISO-8601 timestamp of when the user pressed send.
  DateTimeColumn get createdAt => dateTime().withDefault(currentDateAndTime)();

  /// Number of delivery attempts.
  IntColumn get attempts => integer().withDefault(const Constant(0))();
}
