package main

import (
    "errors"
    "log"
    "os"
    "path/filepath"
    "runtime"

    "github.com/shumatech/wiimplay/config"
    "github.com/shumatech/wiimplay/ui"

    "github.com/diamondburned/gotk4-adwaita/pkg/adw"
    "github.com/diamondburned/gotk4/pkg/gio/v2"
)

var (
    Version string
    Build string
)

func fatalErr(err error) {
    if err != nil {
        log.Fatal(err.Error())
    }
}

func main() {
    runtime.LockOSThread()

    log.Printf("WiiM Play %s (Build: %s)", Version, Build)

    const appID = "com.shumatech.wiimplay"

    userConfig, err := os.UserConfigDir()
    fatalErr(err)
    configDir := filepath.Join(userConfig, "wiimplay")
    _, err = os.Stat(configDir)
    if errors.Is(err, os.ErrNotExist) {
        err = os.MkdirAll(configDir, 0755)
    }
    fatalErr(err)

    config, err := config.NewConfig(configDir)
    fatalErr(err)

    application := adw.NewApplication(appID, gio.ApplicationFlagsNone)

    var controller *ui.Controller
    application.ConnectActivate(func() {
        // Si la ventana estaba oculta (Hide on close), volver a mostrarla
        // en lugar de crear un segundo controlador.
        if controller != nil {
            controller.Show()
            return
        }
        controller, err = ui.NewController(&application.Application, config, Version, Build)
        fatalErr(err)
    })

    os.Exit(application.Run(os.Args))
}
