[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

# WiiM Play (GNOME / GTK4 fork)

A UPnP control point for [WiiM music streamers](https://www.wiimhome.com/)
(Linkplay Technology Inc.) on Linux, with a GNOME-style interface.

> **This is a modified fork of [WiiM Play by ShumaTech](https://github.com/shumatech/wiimplay).**
> The original project, its design, and the device/UPnP code (`upnp/`, `wiimdev/`,
> `mpris/`, `dcache/`, `config/`) are the work of its author; this fork builds on
> it under the same license (GPL-3.0). This fork is not affiliated with or
> endorsed by the original author, WiiM, or Linkplay.

<!-- Add a screenshot of the new interface here, e.g. docs/screenshot.png -->

## What this fork changes

* **GTK4 + libadwaita interface** (via [gotk4](https://github.com/diamondburned/gotk4)) replacing the GTK3 UI: header bars, split view, adaptive light/dark theme, `AdwPreferencesDialog` for settings.
* **Local folder playback** (`localmedia/`): pick a folder on this PC; the app scans it, serves it over HTTP on your LAN and asks the WiiM to play it as a queue.
* **Source panel**: when the WiiM is not playing from a queue, shows the source (Spotify, SoundCloud, TIDAL, ...) reported by the device.
* **Artist info** (`artistinfo/`): most popular albums and songs of the current artist, from Deezer's public API (optional, see below).
* Desktop notifications through `GNotification`; each SOAP request uses its own HTTP connection, which avoids `EOF` errors seen when talking to a WiiM Mini.
* Removed: the status tray icon (GTK4 has no `GtkStatusIcon`, and GNOME shows no tray by default).

## Features

* Automatic discovery of WiiM devices on the network
* View and control playback: transport, seek, volume/mute, loop, shuffle, equalizer, device settings (output, balance, fade, fixed volume)
* Track metadata, stream quality, album art, and the current play queue with thumbnails
* Switch between multiple WiiM devices
* Desktop notifications with album art and track metadata
* MPRIS support - works with media keys, `playerctl`, `osdlyrics`, etc.

By default the window quits when closed; enable **Hide on close** in Settings to keep it running in the background (use the MPRIS controls or launch the app again to bring the window back).

## Build and install

Requires Go >= 1.24, GTK 4 and libadwaita >= 1.5 (`AdwPreferencesDialog`, `AdwAboutDialog`). Optional: `playerctl`, to control playback from media keys (see MPRIS).

On Debian/Ubuntu:

    sudo apt install golang-go build-essential pkg-config libgtk-4-dev libadwaita-1-dev libgirepository1.0-dev
    # libgirepository1.0-dev provides gobject-introspection-1.0.pc; the package name may differ on other distributions
    go mod tidy
    make
    make install        # installs to ~/.local (binary + .desktop file)

The `.desktop` file is needed on GNOME for notifications and for the window to
be associated with the application. Run with `WIIMPLAY_DEBUG=1 wiimplay` to log
extra diagnostics and dump the XML exchanged with the device to `/tmp`.

## MPRIS

WiiM Play supports MPRIS player controls and events, so it works with other Linux software like [playerctl](https://github.com/altdesktop/playerctl), [mpris-ctl](https://git.sr.ht/~mariusor/mpris-ctl) or [osdlyrics](https://github.com/osdlyrics/osdlyrics). For example, with the i3 window manager, media keys can be bound with:

    bindsym XF86AudioPlay exec playerctl play-pause
    bindsym XF86AudioNext exec playerctl next
    bindsym XF86AudioPrev exec playerctl previous
    bindsym Control+XF86AudioRaiseVolume exec playerctl volume 0.05+
    bindsym Control+XF86AudioLowerVolume exec playerctl volume 0.05-
    bindsym Control+XF86AudioMute exec playerctl volume 0.0

This feature may be disabled in the settings.

## Play a local folder

The top bar switches between **WiiM** (default) and **Local folder**. In the Local
folder tab, **Choose folder…** (also in the menu as **Play local folder…**) picks a
folder on this PC and lists its tracks; clicking a track plays it. The app
scans it (FLAC, MP3, WAV, M4A, AAC, OGG/Opus, AIFF; tags and cover art are read
when present), serves it over HTTP on your LAN and asks the WiiM to play it as a
queue.

- The PC must stay on and the app running while the music plays.
- The server listens only on the LAN address that reaches the WiiM, serves only
  files of the chosen folder, and rejects non-local clients. The default port is
  8765 (`LocalMediaPort` in the config file); if a firewall is active, allow it
  (e.g. `sudo ufw allow from 192.168.0.0/16 to any port 8765 proto tcp`).
- If the WiiM rejects the queue, only the first track is played and the reason
  is written to the log.

## Source panel and artist info

When the WiiM is not playing from a queue, the WiiM tab shows the source
(Spotify, SoundCloud, TIDAL, ...) and, below it, the current artist's most
popular albums and songs from [Deezer's public API](https://developers.deezer.com/api).
"Popular" means streams on Deezer, not sales. Artists must match a Deezer artist
by exact name, so uploader names on some services (e.g. SoundCloud) may find
nothing. It can be turned off in Settings → Artist info; when enabled, the
artist name is sent to `api.deezer.com`. Nothing is shown offline. Please
follow Deezer's own terms of use for the API.

## Credits

* **WiiM Play** - original author and copyright holder: **ShumaTech**, © 2024 — <https://github.com/shumatech/wiimplay>
* **GTK4/libadwaita port and the `localmedia`, `artistinfo` and source-panel additions** were developed with the assistance of **Claude**, an AI assistant by [Anthropic](https://www.anthropic.com/), used as a coding tool. As with any AI-assisted code, review and test it before relying on it.
* [gotk4](https://github.com/diamondburned/gotk4) and [gotk4-adwaita](https://github.com/diamondburned/gotk4-adwaita) by diamondburned — GTK4/libadwaita bindings.
* [dhowden/tag](https://github.com/dhowden/tag) by David Howden — audio tag reading.
* Other dependencies from the original project are listed in `go.mod`.
* Artist data: [Deezer](https://www.deezer.com/).

## License

This fork is distributed under the **GNU General Public License v3.0**, the same
license as the original project (see [LICENSE](LICENSE)). Original code:
Copyright (C) 2024 ShumaTech. Modifications and additions: Copyright (C) 2026
the contributors of this fork.

gotk4 and gotk4-adwaita are licensed under the **GNU Affero General Public
License v3.0**; dhowden/tag under a BSD-style license (see its repository). GPL-3.0
and AGPL-3.0 are designed to be combined (section 13 of each license), but if
you distribute binaries of this program, read the terms of both. This is not
legal advice.
