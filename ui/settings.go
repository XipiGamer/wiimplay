package ui

import (
    "github.com/diamondburned/gotk4-adwaita/pkg/adw"
    "github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type Settings struct {
    SendNotifications bool
    MprisSupport bool
    HideOnClose bool
    HideOnStart bool
    ShowArtistInfo bool
}

// Muestra los ajustes en un AdwPreferencesDialog. Los cambios se aplican al
// instante (estilo GNOME); "changed" se invoca tras cada interruptor.
func ShowSettingsDialog(parent gtk.Widgetter, settings *Settings, changed func()) {
    dialog := adw.NewPreferencesDialog()
    dialog.SetTitle("Settings")

    page := adw.NewPreferencesPage()
    group := adw.NewPreferencesGroup()

    addSwitch := func(title string, subtitle string, value *bool) {
        row := adw.NewSwitchRow()
        row.SetTitle(title)
        row.SetSubtitle(subtitle)
        row.SetActive(*value)
        row.Connect("notify::active", func() {
            *value = row.Active()
            changed()
        })
        group.Add(row)
    }

    addSwitch("Send notifications",
        "Show the current track in a desktop notification",
        &settings.SendNotifications)
    addSwitch("MPRIS support",
        "Allow media keys and the system media controls to control WiiM Play",
        &settings.MprisSupport)
    addSwitch("Hide on close",
        "Keep running in the background when the window is closed",
        &settings.HideOnClose)
    addSwitch("Hide on start",
        "Start without showing the window",
        &settings.HideOnStart)

    addSwitch("Artist info",
        "Show the current artist's most popular albums and songs (looked up on Deezer; sends the artist name over the internet)",
        &settings.ShowArtistInfo)

    page.Add(group)
    dialog.Add(page)
    dialog.Present(parent)
}
