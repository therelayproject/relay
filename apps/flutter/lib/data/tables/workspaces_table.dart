import 'package:drift/drift.dart';

/// Local cache of workspaces the current user belongs to.
class WorkspacesTable extends Table {
  @override
  String get tableName => 'workspaces';

  /// Snowflake ID from the server (stored as text — Dart int is 64-bit but
  /// JSON decoding loses precision; we keep IDs as strings in the local DB).
  TextColumn get id => text()();
  TextColumn get name => text()();
  TextColumn get slug => text()();
  TextColumn get iconUrl => text().nullable()();
  BoolColumn get isFederated => boolean().withDefault(const Constant(false))();
  DateTimeColumn get syncedAt => dateTime().withDefault(currentDateAndTime)();

  @override
  Set<Column> get primaryKey => {id};
}
