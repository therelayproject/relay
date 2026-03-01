import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'config.dart';

part 'router.g.dart';

// ── Route paths ───────────────────────────────────────────────────────────────

class Routes {
  static const splash      = '/';
  static const login       = '/login';
  static const register    = '/register';
  static const workspace   = '/w/:workspaceId';
  static const channel     = '/w/:workspaceId/c/:channelId';
  static const dm          = '/w/:workspaceId/dm/:userId';
  static const search      = '/w/:workspaceId/search';
  static const settings    = '/w/:workspaceId/settings';
  static const adminPanel  = '/w/:workspaceId/admin';
}

// ── Placeholder screens (replace with real implementations) ───────────────────

class SplashScreen extends StatelessWidget {
  const SplashScreen({super.key});
  @override
  Widget build(BuildContext context) => const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      );
}

class LoginScreen extends StatelessWidget {
  const LoginScreen({super.key});
  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: const Text('Sign in to Relay')),
        body: const Center(child: Text('Login — TODO')),
      );
}

class RegisterScreen extends StatelessWidget {
  const RegisterScreen({super.key});
  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: const Text('Create account')),
        body: const Center(child: Text('Register — TODO')),
      );
}

class WorkspaceScreen extends StatelessWidget {
  const WorkspaceScreen({super.key, required this.workspaceId});
  final String workspaceId;
  @override
  Widget build(BuildContext context) => Scaffold(
        body: Center(child: Text('Workspace $workspaceId — TODO')),
      );
}

class ChannelScreen extends StatelessWidget {
  const ChannelScreen({
    super.key,
    required this.workspaceId,
    required this.channelId,
  });
  final String workspaceId;
  final String channelId;
  @override
  Widget build(BuildContext context) => Scaffold(
        body: Center(child: Text('Channel $channelId — TODO')),
      );
}

// ── Router ────────────────────────────────────────────────────────────────────

@riverpod
GoRouter router(RouterRef ref) {
  return GoRouter(
    initialLocation: Routes.splash,
    debugLogDiagnostics: true,
    routes: [
      GoRoute(
        path: Routes.splash,
        builder: (_, __) => const SplashScreen(),
      ),
      GoRoute(
        path: Routes.login,
        builder: (_, __) => const LoginScreen(),
      ),
      GoRoute(
        path: Routes.register,
        builder: (_, __) => const RegisterScreen(),
      ),
      GoRoute(
        path: Routes.workspace,
        builder: (_, state) => WorkspaceScreen(
          workspaceId: state.pathParameters['workspaceId']!,
        ),
        routes: [
          GoRoute(
            path: 'c/:channelId',
            builder: (_, state) => ChannelScreen(
              workspaceId: state.pathParameters['workspaceId']!,
              channelId: state.pathParameters['channelId']!,
            ),
          ),
        ],
      ),
    ],
  );
}
