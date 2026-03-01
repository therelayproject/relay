import 'package:flutter/material.dart';

/// Relay brand colours
class RelayColors {
  static const primary    = Color(0xFF5865F2); // indigo
  static const secondary  = Color(0xFF57F287); // green
  static const error      = Color(0xFFED4245); // red
  static const warning    = Color(0xFFFEE75C); // yellow
  static const external   = Color(0xFFFAA61A); // amber — external user badge

  // Dark surface palette
  static const surface900 = Color(0xFF0F1117);
  static const surface800 = Color(0xFF1A1D24);
  static const surface700 = Color(0xFF24272E);
  static const surface600 = Color(0xFF2E3138);

  // Light surface palette
  static const surfaceLight = Color(0xFFFFFFFF);
  static const bgLight      = Color(0xFFF2F3F5);
}

class RelayTheme {
  RelayTheme._();

  static ThemeData light() => ThemeData(
        useMaterial3: true,
        brightness: Brightness.light,
        colorScheme: ColorScheme.fromSeed(
          seedColor: RelayColors.primary,
          brightness: Brightness.light,
        ),
        appBarTheme: const AppBarTheme(
          backgroundColor: RelayColors.surfaceLight,
          foregroundColor: Colors.black87,
          elevation: 0,
          scrolledUnderElevation: 1,
        ),
        inputDecorationTheme: InputDecorationTheme(
          filled: true,
          fillColor: RelayColors.bgLight,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide: BorderSide.none,
          ),
        ),
        extensions: const [RelayThemeExtension.light()],
      );

  static ThemeData dark() => ThemeData(
        useMaterial3: true,
        brightness: Brightness.dark,
        colorScheme: ColorScheme.fromSeed(
          seedColor: RelayColors.primary,
          brightness: Brightness.dark,
          surface: RelayColors.surface800,
        ),
        scaffoldBackgroundColor: RelayColors.surface900,
        appBarTheme: const AppBarTheme(
          backgroundColor: RelayColors.surface800,
          foregroundColor: Colors.white,
          elevation: 0,
          scrolledUnderElevation: 1,
        ),
        inputDecorationTheme: InputDecorationTheme(
          filled: true,
          fillColor: RelayColors.surface700,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
            borderSide: BorderSide.none,
          ),
        ),
        extensions: const [RelayThemeExtension.dark()],
      );
}

/// Custom theme extension for Relay-specific tokens
@immutable
class RelayThemeExtension extends ThemeExtension<RelayThemeExtension> {
  const RelayThemeExtension({
    required this.sidebarBackground,
    required this.channelItemHover,
    required this.externalBadgeColor,
    required this.federatedChannelIcon,
  });

  const RelayThemeExtension.light()
      : sidebarBackground   = RelayColors.bgLight,
        channelItemHover    = const Color(0xFFE3E5E8),
        externalBadgeColor  = RelayColors.external,
        federatedChannelIcon = RelayColors.primary;

  const RelayThemeExtension.dark()
      : sidebarBackground   = RelayColors.surface800,
        channelItemHover    = RelayColors.surface700,
        externalBadgeColor  = RelayColors.external,
        federatedChannelIcon = RelayColors.secondary;

  final Color sidebarBackground;
  final Color channelItemHover;
  final Color externalBadgeColor;   // amber badge for external/federated users
  final Color federatedChannelIcon; // 🔗 tint for federated channels

  @override
  RelayThemeExtension copyWith({
    Color? sidebarBackground,
    Color? channelItemHover,
    Color? externalBadgeColor,
    Color? federatedChannelIcon,
  }) =>
      RelayThemeExtension(
        sidebarBackground:    sidebarBackground   ?? this.sidebarBackground,
        channelItemHover:     channelItemHover    ?? this.channelItemHover,
        externalBadgeColor:   externalBadgeColor  ?? this.externalBadgeColor,
        federatedChannelIcon: federatedChannelIcon ?? this.federatedChannelIcon,
      );

  @override
  RelayThemeExtension lerp(RelayThemeExtension? other, double t) {
    if (other == null) return this;
    return RelayThemeExtension(
      sidebarBackground:    Color.lerp(sidebarBackground,    other.sidebarBackground,    t)!,
      channelItemHover:     Color.lerp(channelItemHover,     other.channelItemHover,     t)!,
      externalBadgeColor:   Color.lerp(externalBadgeColor,   other.externalBadgeColor,   t)!,
      federatedChannelIcon: Color.lerp(federatedChannelIcon, other.federatedChannelIcon, t)!,
    );
  }
}
