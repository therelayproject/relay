import 'package:drift/drift.dart';

/// Local cache of users the current client has encountered.
class UsersTable extends Table {
  @override
  String get tableName => 'users';

  TextColumn get id => text()();
  TextColumn get displayName => text()();
  TextColumn get email => text()();
  TextColumn get avatarUrl => text().nullable()();

  /// 'online' | 'away' | 'dnd' | 'offline'
  TextColumn get presenceStatus =>
      text().withDefault(const Constant('offline'))();

  /// 'internal' | 'external' | 'blocked' — from workspace_domain_policies.
  TextColumn get classification =>
      text().withDefault(const Constant('internal'))();

  /// Home server domain for federated users, null for local users.
  TextColumn get homeServer => text().nullable()();

  DateTimeColumn get syncedAt => dateTime().withDefault(currentDateAndTime)();

  @override
  Set<Column> get primaryKey => {id};
}
