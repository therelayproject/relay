import 'dart:io';
import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:path_provider/path_provider.dart';
import 'package:path/path.dart' as p;

import 'tables/workspaces_table.dart';
import 'tables/channels_table.dart';
import 'tables/messages_table.dart';
import 'tables/users_table.dart';
import 'tables/outbox_table.dart';

part 'database.g.dart';

/// Maximum number of messages cached per channel.
/// Older messages are evicted when this limit is exceeded.
const kMessageCacheLimit = 500;

@DriftDatabase(tables: [
  WorkspacesTable,
  ChannelsTable,
  MessagesTable,
  UsersTable,
  OutboxTable,
])
class AppDatabase extends _$AppDatabase {
  AppDatabase() : super(_openConnection());

  @override
  int get schemaVersion => 1;

  // ── Messages ──────────────────────────────────────────────────────────────

  /// Inserts or updates a message and evicts old rows if the channel exceeds
  /// [kMessageCacheLimit].
  Future<void> upsertMessage(MessagesTableCompanion message) async {
    await into(messagesTable).insertOnConflictUpdate(message);
    await _evictOldMessages(message.channelId.value);
  }

  Future<void> _evictOldMessages(String channelId) async {
    final count = await (selectOnly(messagesTable)
          ..addColumns([messagesTable.id.count()])
          ..where(messagesTable.channelId.equals(channelId)))
        .map((r) => r.read(messagesTable.id.count()))
        .getSingle();

    if ((count ?? 0) > kMessageCacheLimit) {
      final oldest = await (select(messagesTable)
            ..where((t) => t.channelId.equals(channelId))
            ..orderBy([(t) => OrderingTerm.asc(t.createdAt)])
            ..limit((count! - kMessageCacheLimit)))
          .map((r) => r.id)
          .get();

      await (delete(messagesTable)
            ..where((t) => t.id.isIn(oldest)))
          .go();
    }
  }

  /// Returns cached messages for a channel ordered oldest→newest.
  Future<List<MessagesTableData>> getMessages(String channelId) =>
      (select(messagesTable)
            ..where((t) => t.channelId.equals(channelId))
            ..orderBy([(t) => OrderingTerm.asc(t.createdAt)]))
          .get();

  // ── Outbox ────────────────────────────────────────────────────────────────

  Future<List<OutboxTableData>> getPendingOutbox() =>
      (select(outboxTable)
            ..orderBy([(t) => OrderingTerm.asc(t.createdAt)]))
          .get();

  Future<void> deleteOutboxItem(int id) =>
      (delete(outboxTable)..where((t) => t.id.equals(id))).go();
}

LazyDatabase _openConnection() {
  return LazyDatabase(() async {
    final dir = await getApplicationDocumentsDirectory();
    final file = File(p.join(dir.path, 'relay.sqlite'));
    return NativeDatabase.createInBackground(file);
  });
}
