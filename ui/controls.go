package ui

import (
    "github.com/diamondburned/gotk4-adwaita/pkg/adw"
    "github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type Controls struct {
    AudioOutput int
    AudioOutputList []string
    AudioInput int
    AudioInputList []string
    Balance float64
    FadeEffects bool
    FixedVolume bool
}

type ControlsListener interface {
    AudioOutputSelect(index int)
    AudioInputSelect(index int)
    BalanceSelect(value float64)
    FadeEffectsSelect(on bool)
    FixedVolumeSelect(on bool)
}

func newComboRow(title string, items []string, selected int,
                 onSelect func(index int)) *adw.ComboRow {
    row := adw.NewComboRow()
    row.SetTitle(title)
    row.SetModel(gtk.NewStringList(items))
    if selected >= 0 && selected < len(items) {
        row.SetSelected(uint(selected))
    }
    // Se conecta después de fijar el valor inicial para no disparar el callback.
    row.Connect("notify::selected", func() {
        onSelect(int(row.Selected()))
    })
    return row
}

func newSwitchRow(title string, active bool, onToggle func(on bool)) *adw.SwitchRow {
    row := adw.NewSwitchRow()
    row.SetTitle(title)
    row.SetActive(active)
    row.Connect("notify::active", func() {
        onToggle(row.Active())
    })
    return row
}

func ShowControlsDialog(parent gtk.Widgetter, controls *Controls,
                        listener ControlsListener) {
    dialog := adw.NewPreferencesDialog()
    dialog.SetTitle("Device controls")

    page := adw.NewPreferencesPage()

    audio := adw.NewPreferencesGroup()
    audio.SetTitle("Audio")
    audio.Add(newComboRow("Audio output", controls.AudioOutputList,
        controls.AudioOutput, listener.AudioOutputSelect))
    audio.Add(newComboRow("Audio input", controls.AudioInputList,
        controls.AudioInput, listener.AudioInputSelect))

    adjust := gtk.NewAdjustment(controls.Balance, -1.0, 1.0, 0.05, 0.1, 0.0)
    scale := gtk.NewScale(gtk.OrientationHorizontal, adjust)
    scale.SetDrawValue(false)
    scale.SetHasOrigin(false)
    scale.SetSizeRequest(220, -1)
    scale.SetVAlign(gtk.AlignCenter)
    adjust.ConnectValueChanged(func() {
        listener.BalanceSelect(adjust.Value())
    })
    balance := adw.NewActionRow()
    balance.SetTitle("Volume balance")
    balance.AddSuffix(scale)
    audio.Add(balance)
    page.Add(audio)

    behavior := adw.NewPreferencesGroup()
    behavior.SetTitle("Behavior")
    behavior.Add(newSwitchRow("Fade effects", controls.FadeEffects,
        listener.FadeEffectsSelect))
    behavior.Add(newSwitchRow("Fixed volume", controls.FixedVolume,
        listener.FixedVolumeSelect))
    page.Add(behavior)

    dialog.Add(page)
    dialog.Present(parent)
}
