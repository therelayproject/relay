import 'package:drift/drift.dart';

class ChannelsTable extends Table {
  @override
  String get tableName => 'channels';

  TextColumn get id => text()();
  TextColumn get workspaceId => text()();
  TextColumn get name => text()();

  /// 'public' | 'private' | 'dm' | 'thread'
  TextColumn get type => text().withDefault(const Constant('public'))();

  TextColumn get topic => text().nullable()();
  BoolColumn get isFederated => boolean().withDefault(const Constant(false))();

  /// Last message Snowflake ID the current user has read (for unread badge).
  TextColumn get lastReadId => text().nullable()();
  IntColumn get mentionCount => integer().withDefault(const Constant(0))();

  DateTimeColumn get syncedAt => dateTime().withDefault(currentDateAndTime)();

  @override
  Set<Column> get primaryKey => {id};
}
