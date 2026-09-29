# Shortr for Android

Native Android client for Shortr, written in Kotlin with Jetpack Compose and
Material 3. User documentation lives in the wiki:
[Android App](../docs/wiki/Android-App.md).

## Requirements

- JDK 17 or newer (the JDK bundled with Android Studio works)
- Android SDK with platform 37 (Android Studio installs it on first sync)

## Build

```bash
cd android
./gradlew assembleDebug        # app/build/outputs/apk/debug/app-debug.apk
./gradlew testDebugUnitTest    # unit tests
./gradlew lintDebug            # Android lint
```

Or open the `android` folder in Android Studio and press Run.

Install on a connected phone or emulator:

```bash
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

To reach a Shortr server running on your computer from the emulator, use
`http://10.0.2.2:8080`.

## Signed release builds

Create a keystore once and keep it safe: updates must be signed with the
same key.

```bash
keytool -genkeypair -v -keystore shortr-release.jks -alias shortr \
  -keyalg RSA -keysize 4096 -validity 10000
```

Then create `android/keystore.properties` (git-ignored):

```properties
storeFile=shortr-release.jks
storePassword=…
keyAlias=shortr
keyPassword=…
```

and build:

```bash
./gradlew assembleRelease      # app/build/outputs/apk/release/app-release.apk
```

Without a keystore the release APK is unsigned and can't be installed; use
the debug build instead.

### In GitHub Actions

The release workflow builds a signed release APK from these repository secrets:

| Secret | Value |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | `base64 -w0 shortr-release.jks` |
| `ANDROID_KEYSTORE_PASSWORD` | keystore password |
| `ANDROID_KEY_ALIAS` | key alias |
| `ANDROID_KEY_PASSWORD` | key password |

Without them the release job fails rather than publish an unsigned or debug APK, since
a debug-signed APK could not be updated by a later properly signed build.

## Code layout

```
app/src/main/java/io/github/anand34577/shortr/
  data/        API client, models, encrypted storage, SSO (AppAuth)
  ui/home      dashboard
  ui/links     list, detail and editor
  ui/onboarding  connect, pairing-code scanner
  ui/settings  settings
  ui/lock      app lock
  ui/common    shared components, chart, formatting
```

There is no DI framework: `AppContainer` holds the few shared objects and
view models get it through `appViewModel { … }`.

## Server endpoints the app relies on

- `GET /api/v1/client-config`: unauthenticated; confirms the address is
  Shortr and advertises SSO settings
- `/api/v1/me`, `/api/v1/links…`, `/api/v1/stats…`: authenticated with
  `Authorization: Bearer <api key or SSO access token>`

Pairing QR codes encode `shortr://connect?server=<url>&key=<sk_…>`.
