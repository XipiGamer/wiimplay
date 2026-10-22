package ui

import (
    "github.com/diamondburned/gotk4-adwaita/pkg/adw"
    "github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func ShowAboutDialog(parent gtk.Widgetter, about string) error {
    dialog := adw.NewAboutDialog()
    dialog.SetApplicationName("WiiM Play")
    dialog.SetDeveloperName("ShumaTech")
    dialog.SetCopyright("© 2024 ShumaTech")
    dialog.SetLicenseType(gtk.LicenseGPL30)
    dialog.SetComments(about + "\n\nBased on WiiM Play by ShumaTech\ngithub.com/shumatech/wiimplay")
    dialog.Present(parent)
    return nil
}
