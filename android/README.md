# Polaris for Android

Polaris in a window of its own instead of a browser tab. The app shows the pages of
your Polaris server, so it never needs rebuilding when Polaris is updated. Without a
connection it does what the site does: a story kept for reading offline (the story
picker's "…" menu, "Keep for reading offline") opens whole, and books kept for writing
offline can be written in.

What it adds to a bare web view: it stays signed in, picks pictures and files for
uploads, saves exports and story `.zip`s to Downloads, prints, follows the Back button,
opens outside links in the browser, and says so when the server can't be reached.

Everything is in [`MainActivity.kt`](app/src/main/java/net/astralumi/polaris/MainActivity.kt).

## Build

Needs a JDK 17 and the Android SDK (platform 35).

1. Copy `secrets.example.properties` to `secrets.properties` and set `POLARIS_URL` to
   your server's address (`https://…`). Left empty, the app asks for it on first start.
2. Write `local.properties` with the SDK's place: `sdk.dir=/path/to/android/sdk`.
3. Build:

   ```bash
   cd android
   JAVA_HOME=/path/to/jdk17 ./gradlew assembleRelease
   ```

The APK is `app/build/outputs/apk/release/app-release.apk`, signed with the debug key of
the computer that built it (a personal, sideloaded app). Install it with
`adb install -r <apk>` or copy it to the phone. Its version is the one in
`frontend/package.json`.

Both files above are ignored by git: the server's address never goes in the repository.

## Signing in behind Cloudflare Access

The app only ever shows two places: your server and Cloudflare Access's sign-in page
(`<team>.cloudflareaccess.com`). Any other link opens in the phone's browser.

- **A code by email (one-time PIN)** works as it is: the sign-in page appears in the app
  the first time, and the app stays signed in for the application's *session duration*.
  Set that to a month in Zero Trust → Access controls → Applications → your application
  so the phone rarely asks.
- **Google sign-in does not work** in any app's web view (Google refuses it). Add the
  one-time PIN as a second login method for the application.
- Service tokens are not used: Access wants their two headers on every request, which
  a web view can't add to the page's own requests.

When the sign-in runs out while the app is open, a bar at the bottom offers to sign in
again.

## Limits

- The camera can't be used straight from a file picker (pick a photo already taken).
- "Open in a new tab" links open in the same window; Back returns.
