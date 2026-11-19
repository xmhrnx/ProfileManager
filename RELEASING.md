# Releasing Profile Manager

Pushing a tag like `v0.2.0` (`git tag v0.2.0 && git push --tags`) runs
`.github/workflows/release.yml`. The tag sets the version and the run number
the build number. The workflow runs `go vet` and `go test`, builds every
package and attaches them to a GitHub release. "Run workflow" in the Actions
tab builds them without releasing (download them from the run's artifacts).

The workflow is written by the Giode CLI from `giode.toml`: after changing
`[ci]` or a store section, run `giode ci -f` instead of editing it, then redo
the two pins by hand (the CLI at `giode-cli/cmd/giode@v0.1.35-alpha.1` instead
of `@latest`, and `go-version-file: go.mod` instead of `go-version: stable`).

| Where | What the release gets | Needs |
| --- | --- | --- |
| GitHub, Linux | `.deb` and `.rpm`, x64 | nothing |
| GitHub, Windows | `ProfileManager-<version>-amd64-setup.exe`, x64 | nothing; a code signing certificate stops SmartScreen warnings |
| GitHub, macOS | a universal `.dmg` | a Developer ID certificate and notarization, or macOS refuses to open it |
| Microsoft Store | `ProfileManager-<version>-amd64.msix`, to upload in Partner Center | a Partner Center account and the reserved name; **read [the MSIX limits](#microsoft-store) first** |
| Mac App Store | not built | the App Sandbox rules out what the app does; see [below](#mac-app-store-not-supported) |

There are no Google Play or iOS builds: Profile Manager is a desktop app
(`platforms = ["windows", "macos", "linux"]`), and no Snap or Flathub package.

## Repository secrets

Set these in GitHub under Settings → Secrets and variables → Actions. Each
`*_BASE64` secret is the file encoded with `base64 -i file | pbcopy` on macOS,
or `base64 -w0 file` on Linux. Without a secret, that step is skipped and the
rest of the release still builds (unsigned).

| Secret | What it is |
| --- | --- |
| `MACOS_CERT_BASE64`, `MACOS_CERT_PASS` | The "Developer ID Application" certificate as a `.p12`, and its password: signs the app and the `.dmg` |
| `GIODE_NOTARY_ID`, `GIODE_NOTARY_TEAM_ID`, `GIODE_NOTARY_PASS` | Your Apple ID, team ID and an app-specific password (appleid.apple.com → Sign-In and Security → App-Specific Passwords): notarizes the `.dmg` |
| `WINDOWS_CERT_BASE64`, `GIODE_WINDOWS_CERT_PASS` | An Authenticode `.pfx` and its password: signs `ProfileManager.exe` and the setup program (optional; the Store signs the MSIX itself) |

No Mac App Store or Android secrets are used.

## macOS: the signed, notarized .dmg

1. Join the Apple Developer Program.
2. In Certificates, Identifiers & Profiles, create a **Developer ID
   Application** certificate. Export it from Keychain Access as a `.p12` with a
   password, and set `MACOS_CERT_BASE64` and `MACOS_CERT_PASS`.
3. Create an app-specific password for your Apple ID and set
   `GIODE_NOTARY_ID` (the Apple ID), `GIODE_NOTARY_TEAM_ID` (the 10-character
   team ID from the membership page) and `GIODE_NOTARY_PASS`.
4. Push a tag. The macOS job signs the app with the Developer ID identity,
   notarizes it and attaches the `.dmg`.

The bundle ID is `com.xmhrnx.profilemanager`; no App ID or provisioning
profile is needed for Developer ID distribution.

Expected behaviour after install, worth putting in the release notes: the
first time Profile Manager reads a Keychain item another program created
(Claude Code's, GitHub CLI's), macOS asks the user to allow it.

## Mac App Store: not supported

Every Mac App Store app runs in the App Sandbox, and the sandbox forbids the
very things Profile Manager exists to do:

- **Other programs' files.** A sandboxed app sees only its own container and
  files the user picks in an open panel. Profile Manager has to read and
  rewrite `~/.aws`, `~/.claude.json`, `~/.kube/config`, `~/.docker`,
  `~/Library/Application Support/Slack` and so on, silently, in the
  background (auto-sync and switches). The only entitlements that reach them
  are `temporary-exception.files.home-relative-path.*`, which App Review
  rejects for this kind of use; picking each folder by hand with
  security-scoped bookmarks would break `~/.claude.json`-style single files
  next to other data, and every app definition the user adds.
- **Other programs' Keychain items.** Claude Code and the GitHub CLI keep
  their tokens in the login keychain under their own access groups. A
  sandboxed app (and `/usr/bin/security`, which it runs as a child and which
  would inherit the sandbox) can only reach its own keychain access groups.
- **Running other programs.** Native switching runs `gh`, `gcloud`, `kubectl`
  and `az`, and `profilemanager exec`/`shell` run the user's commands. Child
  processes inherit the sandbox, so they couldn't read their own config
  either, and Homebrew-installed tools under `/opt/homebrew` may not run at
  all.
- **Quitting other apps.** Switching Slack or Claude Code may send them
  SIGTERM; a sandboxed app can't signal other processes.
- **The command line.** The `profilemanager` CLI is the same binary; a Mac App
  Store app can't put a command on the user's PATH.

A sandboxed build could do nothing useful, so `giode.toml` has no
`[macos.store]` section and the workflow builds no `.pkg`. Mac users get the
notarized `.dmg` from GitHub. (`[macos.plist]` still sets the category, and
`[privacy]` writes a privacy manifest, so nothing is missing if this is ever
revisited.)

## Microsoft Store

### Read this first: MSIX file-system virtualization

The MSIX runs Profile Manager as a packaged full-trust desktop app. Windows
then **virtualizes writes under `%APPDATA%` and `%LOCALAPPDATA%`**: on Windows
10 1903 and later, a packaged app may modify files that already exist there,
but every *new* file or folder it creates there goes to a private per-package
location that only the package (and processes in its container) sees, and is
deleted when the package is uninstalled.

Profile Manager writes files by write-then-rename (a new temporary file
beside the old one) and swaps folders by building a new one beside the live
one. Under `%APPDATA%` those new files and folders are virtualized, so a
switch may look done to Profile Manager while the real program still sees
the old sign-in, or none. That affects every built-in app whose Windows
sign-in lives under `%APPDATA%`:

| Affected in the MSIX (under `%APPDATA%`) | Not affected (in the user folder, or the Credential Manager) |
| --- | --- |
| Slack, Discord, Telegram Desktop, Google Cloud CLI, GitHub CLI, Netlify, Terraform/HCP, Vercel, Wrangler (its `%APPDATA%\xdg.config` location) | AWS, Azure CLI, Claude Code, Codex, Docker, Firebase, Fly.io, Gemini CLI, kubectl, npm, Supabase, Wrangler (`~\.wrangler`); the vault key in Credential Manager |

Other consequences in the MSIX:

- **Profile Manager's own data** (`%APPDATA%\ProfileManager`: the vault and
  `state.json`) lives in the package's private folder and is **deleted on
  uninstall**. The master key in Credential Manager is not, so a reinstall
  starts with an empty vault and an unused key.
- **Per-command runs** (`exec`, `shell`) decrypt into `%APPDATA%\ProfileManager\run`
  and point the program there with environment variables. A program started
  inside the package's container sees that folder; one that isn't may not.
- **No `profilemanager` command.** The package declares no app execution
  alias, so the CLI isn't on PATH in a Store install.

The setup `.exe` from GitHub has none of these limits.

**Before submitting the MSIX, test it on Windows** (Developer Mode on, so an
unsigned package can be registered):

```powershell
# from the release's .msix: unpack and register it
Expand-Archive ProfileManager-<version>-amd64.msix -DestinationPath pm-msix
Add-AppxPackage -Register .\pm-msix\AppxManifest.xml
```

Then, for one affected app (GitHub CLI: `gh auth login` with two accounts)
and one unaffected app (AWS or Claude Code): save two profiles, switch,
and check the real program (`gh auth status`) sees the switched account;
quit and relaunch Profile Manager; uninstall with
`Get-AppxPackage *ProfileManager* | Remove-AppxPackage`.

If the affected apps don't switch, there are two ways forward:

1. **Submit the setup program instead of the MSIX.** Partner Center accepts
   unpackaged Win32 apps as an "EXE or MSI app": you give an HTTPS URL to the
   versioned, signed `ProfileManager-<version>-amd64-setup.exe` (the GitHub
   release asset), and the setup must run silently with `/S` (it does). It
   needs the installer signed with a code signing certificate from a CA
   (`WINDOWS_CERT_BASE64`, `GIODE_WINDOWS_CERT_PASS`). No virtualization
   applies. This is the recommended route for this app.
2. **Turn virtualization off in the MSIX.** The manifest can declare
   `<desktop6:FileSystemWriteVirtualization>disabled</desktop6:FileSystemWriteVirtualization>`
   with the restricted capability `unvirtualizedResources`, which the Store
   approves case by case. The Giode CLI (v0.1.35-alpha.1) can't write those
   yet, so this needs a CLI change first.

### Submitting the MSIX

1. In Partner Center (the same account as EasySend and AnyFile), reserve the
   name **Profile Manager**.
2. `giode.toml` already has the account's values under `[windows.msix]`:
   `publisher = "CN=B40E8F57-A7E0-40BB-8E74-5D051D0B578B"` and
   `publisher_display_name = "Mehran Zahiroddini"`. `identity_name` is set to
   `MehranZahir.ProfileManager`: once the name is reserved, open the product's
   **Product identity** page and check that **Package/Identity/Name** matches
   it exactly; if not, change it, commit, and tag again. (If "Profile Manager"
   is taken and you reserve another name, set that as `identity_name` and make
   the MSIX's display name match it.)
3. Push a tag. The release gets `ProfileManager-<version>-amd64.msix`; upload
   it in a submission. The Store signs it, so it needs no certificate here.
4. **Restricted capability `runFullTrust`:** explain that Profile Manager is a
   desktop app that saves and restores other programs' sign-in files and
   Windows Credential Manager entries, runs their command-line tools, and
   offers a command-line interface.
5. **Privacy policy URL:** https://xmhrnx.github.io/ProfileManager/#privacy
6. **Age rating (IARC questionnaire):** a utility; no violence, sexual
   content, gambling or user-to-user communication; it shares no location or
   personal data; no purchases. That comes out as the lowest rating.
7. **Category:** Developer tools.

## Store listing

**Name:** Profile Manager

**Short description (≤ 80 characters):**
Switch accounts in CLIs and desktop apps in one click, no signing out.

**Search terms (Microsoft Store, up to 7):**
account switcher, multiple accounts, profiles, aws, claude code, wrangler, slack

**Description:**

> Profile Manager switches the account a program is signed in to without
> signing out and back in. Keep a work and a personal Cloudflare login, two
> Claude Code accounts, separate AWS or Slack setups: each is a profile, and
> switching takes one click from the window or the tray.
>
> Twenty apps are built in: Cloudflare Wrangler, AWS CLI, Google Cloud CLI,
> Azure CLI, Vercel, Netlify, Firebase, Fly.io, Supabase, Terraform, Claude
> Code, OpenAI Codex CLI, Gemini CLI, GitHub CLI, kubectl, Docker, npm, Slack,
> Discord and Telegram Desktop. Add any other program with a short form or a
> JSON definition.
>
> Three ways to switch: swap the files and credential entries that hold the
> sign-in, call the program's own profile commands, or run a single command
> (or a whole shell) signed in to another profile while everything else stays
> as it is.
>
> Profiles stay current: the live sign-in is saved before each switch, and
> auto-sync keeps rotating tokens up to date. Switches are atomic and roll
> back if anything fails, and a sign-in that belongs to another account is
> never written over your profile.
>
> Saved sign-ins are encrypted with AES-256-GCM, with the key in the system
> credential store. Profile Manager has no accounts, no analytics and sends
> nothing anywhere.

Add for the Microsoft Store if it ships as the MSIX: "In the Microsoft Store
version, the profilemanager command-line tool isn't available; get it with
the installer from github.com/xmhrnx/ProfileManager."

**Data collection (Partner Center / any store's privacy questions):** no data
collected, none shared, no tracking. Everything stays on the computer; see the
privacy policy.

**Export compliance:** the app encrypts only the user's own data at rest, with
AES-256-GCM from Go's standard library. The Microsoft Store asks no export
question for that; if a store does, it is standard cryptography for data
protection.

**Screenshots:** `docs/screenshot-dark.png` (1240 × 800, also on the
website) shows the main window with three Wrangler profiles. The Microsoft
Store wants desktop screenshots of at least 1366 × 768: capture new ones at
1920 × 1080 with a few demo profiles (main window dark, the same in light,
the per-command terminal panel, Add your own app). Don't capture real
account emails or tokens.

## Linux

The `.deb` and `.rpm` are attached to the release, and the website's download
section links to the release. They list the system libraries they need, so
`apt` and `dnf` install those too.
