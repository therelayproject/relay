import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/config.dart';
import 'app/router.dart';
import 'app/theme.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(
    const ProviderScope(
      child: RelayApp(),
    ),
  );
}

class RelayApp extends ConsumerWidget {
  const RelayApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);

    return MaterialApp.router(
      title: AppConfig.appName,
      debugShowCheckedModeBanner: false,
      theme: RelayTheme.light(),
      darkTheme: RelayTheme.dark(),
      themeMode: ThemeMode.system,
      routerConfig: router,
    );
  }
}
