import 'package:drift/drift.dart';

/// Local cache of messages. Stores a rolling window of the most recent
/// 500 messages per channel (older rows are evicted on insert).
class MessagesTable extends Table {
  @override
  String get tableName => 'messages';

  TextColumn get id => text()();
  TextColumn get channelId => text()();
  TextColumn get authorId => text()();
  TextColumn get content => text()();

  /// Raw Block Kit JSON, null for plain-text messages.
  TextColumn get blocks => text().nullable()();

  /// ID of the parent message if this is a thread reply.
  TextColumn get parentId => text().nullable()();

  /// 'pending' | 'sent' | 'failed'  — local-only optimistic state.
  TextColumn get localStatus =>
      text().withDefault(const Constant('sent'))();

  DateTimeColumn get createdAt => dateTime()();
  DateTimeColumn get updatedAt => dateTime().nullable()();

  @override
  Set<Column> get primaryKey => {id};

  @override
  List<String> get customConstraints => [
        'FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE',
      ];
}
