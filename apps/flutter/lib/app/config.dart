/// App configuration resolved from --dart-define compile-time variables.
///
/// Build with a flavor:
///   flutter run --dart-define=FLAVOR=dev
///   flutter run --dart-define=FLAVOR=staging
///   flutter run --dart-define=FLAVOR=prod
///   flutter run --dart-define=RELAY_API_URL=https://my-server.com
class AppConfig {
  static const _flavor = String.fromEnvironment('FLAVOR', defaultValue: 'dev');
  static const _apiUrl = String.fromEnvironment('RELAY_API_URL', defaultValue: '');

  static Flavor get flavor => switch (_flavor) {
        'prod' => Flavor.prod,
        'staging' => Flavor.staging,
        _ => Flavor.dev,
      };

  static String get apiUrl {
    if (_apiUrl.isNotEmpty) return _apiUrl;
    return switch (flavor) {
      Flavor.prod => 'https://relay.gg',
      Flavor.staging => 'https://staging.relay.gg',
      Flavor.dev => 'http://localhost:8081',
    };
  }

  static String get wsUrl => apiUrl
      .replaceFirst('https://', 'wss://')
      .replaceFirst('http://', 'ws://');

  static bool get isDebug => flavor == Flavor.dev;

  static String get appName => switch (flavor) {
        Flavor.prod => 'Relay',
        Flavor.staging => 'Relay (Staging)',
        Flavor.dev => 'Relay (Dev)',
      };
}

enum Flavor { dev, staging, prod }
