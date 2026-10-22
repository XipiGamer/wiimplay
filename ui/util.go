package ui

import (
    "errors"
    "log"
    "net/url"
    "strings"

    "github.com/shumatech/wiimplay/mpris"
    "github.com/shumatech/wiimplay/wiimdev"

    "github.com/diamondburned/gotk4-adwaita/pkg/adw"
    "github.com/diamondburned/gotk4/pkg/gdk/v4"
    "github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
    "github.com/diamondburned/gotk4/pkg/gio/v2"
    "github.com/diamondburned/gotk4/pkg/glib/v2"
    "github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func Sanitize(s string) string {
    reserved := [...][2]string{
        {"&", "&amp;"  },
        {"\"", "&quot;"},
        {"'", "&apos;" },
        {"<", "&lt;"   },
        {">", "&gt;"   },
    }
    for _, r := range reserved {
        s = strings.ReplaceAll(s, r[0], r[1])
    }
    return s
}

func logErr(err error) {
    if err != nil {
        log.Println(err)
    }
}


// Crea un Pixbuf a partir de bytes de imagen. Si w y h son > 0 la imagen se
// escala al decodificarla (se puede llamar fuera del hilo principal).
func pixbufFromData(data []byte, w int, h int) (*gdkpixbuf.Pixbuf, error) {
    loader := gdkpixbuf.NewPixbufLoader()
    if w > 0 && h > 0 {
        loader.SetSize(w, h)
    }
    if err := loader.Write(data); err != nil {
        loader.Close()
        return nil, err
    }
    if err := loader.Close(); err != nil {
        return nil, err
    }
    pixbuf := loader.Pixbuf()
    if pixbuf == nil {
        return nil, errors.New("could not decode image")
    }
    return pixbuf, nil
}

func newScaledPixbufFromData(data []byte, w int, h int) (*gdkpixbuf.Pixbuf, error) {
    return pixbufFromData(data, w, h)
}

func textureFromData(data []byte) (*gdk.Texture, error) {
    pixbuf, err := pixbufFromData(data, 0, 0)
    if err != nil {
        return nil, err
    }
    return gdk.NewTextureForPixbuf(pixbuf), nil
}

// Diálogo modal (AdwAlertDialog) con cabecera y texto.
func noticeDialog(parent gtk.Widgetter, heading string, message string) {
    dialog := adw.NewAlertDialog(heading, message)
    dialog.AddResponse("close", "Close")
    dialog.Present(parent)
}

func messageDialog(parent gtk.Widgetter, message string) {
    noticeDialog(parent, "Error", message)
}

// Schedule a closure to execute on the main thread after a delay.
func mainDelay(millis uint, fn func()) glib.SourceHandle {
    return glib.TimeoutAdd(millis, func() bool {
        fn()
        return false
    })
}

// Schedule a closure to run on the main thread (seguro desde cualquier hilo)
func mainRun(fn func()) {
    glib.IdleAdd(func() bool {
        fn()
        return false
    })
}

// Schedule a closure to execute on the main thread and wait for its return.
// Warning! Do not call from the main thread or it will hang.
func mainAwait(fn func() interface{}) interface{} {
    resultChan := make(chan interface{})
    glib.IdleAdd(func() bool {
        resultChan <- fn()
        return false
    })
    return <-resultChan
}

// Notificación de escritorio vía GNotification. En GNOME requiere que exista
// un .desktop con el mismo ID de aplicación (ver "make install").
func sendNotification(application *gtk.Application, title string,
                      body string, icon gio.Iconner) {
    notification := gio.NewNotification(title)
    notification.SetBody(body)
    if icon != nil {
        notification.SetIcon(icon)
    }
    application.SendNotification("notify", notification)
}

func loopModeToStatus(loopMode wiimdev.LoopMode) (loopStatus mpris.LoopStatus, shuffle bool) {
    switch loopMode {
        case wiimdev.LoopModeAll:
            loopStatus = mpris.LoopStatusPlaylist
            shuffle = false
        case wiimdev.LoopModeOne:
            loopStatus = mpris.LoopStatusTrack
            shuffle = false
        case wiimdev.LoopModeAllShuffle:
            loopStatus = mpris.LoopStatusPlaylist
            shuffle = true
        case wiimdev.LoopModeNoneShuffle:
            loopStatus = mpris.LoopStatusNone
            shuffle = true
        case wiimdev.LoopModeNone:
            loopStatus = mpris.LoopStatusNone
            shuffle = false
        case wiimdev.LoopModeOneShuffle:
            loopStatus = mpris.LoopStatusTrack
            shuffle = true
    }
    return
}

func loopStatusToMode(loopStatus mpris.LoopStatus, shuffle bool) wiimdev.LoopMode {
    if shuffle {
        switch loopStatus {
            case mpris.LoopStatusNone:
                return wiimdev.LoopModeNoneShuffle
            case mpris.LoopStatusTrack:
                return wiimdev.LoopModeOneShuffle
            case mpris.LoopStatusPlaylist:
                return wiimdev.LoopModeAllShuffle
        }
    } else {
        switch loopStatus {
            case mpris.LoopStatusNone:
                return wiimdev.LoopModeNone
            case mpris.LoopStatusTrack:
                return wiimdev.LoopModeOne
            case mpris.LoopStatusPlaylist:
                return wiimdev.LoopModeAll
        }
    }
    return wiimdev.LoopModeNone
}

func loopModeToState(loopMode wiimdev.LoopMode) (loopState LoopState, shuffle bool) {
    switch loopMode {
        case wiimdev.LoopModeAll:
            loopState = LoopAll
            shuffle = false
        case wiimdev.LoopModeOne:
            loopState = LoopOne
            shuffle = false
        case wiimdev.LoopModeAllShuffle:
            loopState = LoopAll
            shuffle = true
        case wiimdev.LoopModeNoneShuffle:
            loopState = LoopNone
            shuffle = true
        case wiimdev.LoopModeNone:
            loopState = LoopNone
            shuffle = false
        case wiimdev.LoopModeOneShuffle:
            loopState = LoopOne
            shuffle = true
    }
    return
}

func loopStateToMode(loopState LoopState, shuffle bool) wiimdev.LoopMode {
    if shuffle {
        switch loopState {
            case LoopNone:
                return wiimdev.LoopModeNoneShuffle
            case LoopOne:
                return wiimdev.LoopModeOneShuffle
            case LoopAll:
                return wiimdev.LoopModeAllShuffle
        }
    } else {
        switch loopState {
            case LoopNone:
                return wiimdev.LoopModeNone
            case LoopOne:
                return wiimdev.LoopModeOne
            case LoopAll:
                return wiimdev.LoopModeAll
        }
    }
    return wiimdev.LoopModeNone
}

func stateTransportToPlayer(state wiimdev.TransportState) PlayerState {
    switch state {
    case wiimdev.StateStopped:
        return PlayerPaused
    case wiimdev.StatePaused:
        return PlayerPaused
    case wiimdev.StatePlaying:
        return PlayerPlaying
    case wiimdev.StateTransitioning:
        return PlayerPlaying
    case wiimdev.StateNoMedia:
        return PlayerPaused
    default:
        return PlayerUnavailable
    }
}

func statePlayerToTransport(state PlayerState) wiimdev.TransportState {
    switch state {
    case PlayerPaused:
        return wiimdev.StatePaused
    case PlayerPlaying:
        return wiimdev.StatePlaying
    default:
        return wiimdev.StateUnknown
    }
}

func urlToHost(urlStr string) string {
    urlParsed, err := url.Parse(urlStr)
    if err != nil {
        return urlStr
    }
    return strings.Split(urlParsed.Host, ":")[0]
}


func metadataEqual(mdA *wiimdev.Metadata, mdB *wiimdev.Metadata) bool {
    return (mdA.Id == mdB.Id &&
            mdA.Title == mdB.Title &&
            mdA.Artist == mdB.Artist &&
            mdA.Album == mdB.Album)
}

func playListsEqual(plA []wiimdev.PlayList, plB []wiimdev.PlayList) bool {
    if len(plA) != len(plB) {
        return false
    }
    for i, pl := range(plA) {
        if pl.Id != plB[i].Id {
            return false
        }
    }
    return true
}

func tracksEqual(trA []wiimdev.Track, trB []wiimdev.Track) bool {
    if len(trA) != len(trB) {
        return false
    }
    for i, tr := range(trA) {
        if !metadataEqual(tr.Metadata, trB[i].Metadata) {
            return false
        }
    }
    return true
}


// Servicios conocidos. Se busca el texto (sin distinguir mayúsculas) dentro de
// lo que informa el dispositivo; si no coincide ninguno, se muestra tal cual.
var knownSources = []struct{ match, name string }{
    {"spotify", "Spotify"},
    {"tidal", "TIDAL"},
    {"soundcloud", "SoundCloud"},
    {"qobuz", "Qobuz"},
    {"deezer", "Deezer"},
    {"amazon", "Amazon Music"},
    {"youtube", "YouTube Music"},
    {"airplay", "AirPlay"},
    {"tunein", "TuneIn"},
    {"iheart", "iHeartRadio"},
    {"pandora", "Pandora"},
    {"napster", "Napster"},
    {"roon", "Roon"},
    {"bluetooth", "Bluetooth"},
    {"upnp", "UPnP / DLNA"},
    {"dlna", "UPnP / DLNA"},
}

// En Spotify, TrackSource es una URI ("spotify:playlist:37i9..."): para
// reconocer el servicio solo se usa lo que va antes de los dos puntos, así un
// identificador aleatorio no puede coincidir con otro servicio.
func sourceToken(s string) string {
    s = strings.TrimSpace(s)
    if i := strings.Index(s, ":"); i > 0 {
        return s[:i]
    }
    return s
}

// Devuelve el nombre de la fuente de reproducción y una descripción, o
// cadenas vacías si el dispositivo no informa ninguna. No se usa SpotifyActive:
// el WiiM lo informa como verdadero también cuando suena otro servicio.
func describeSource(src wiimdev.SourceInfo) (string, string) {
    token := sourceToken(src.TrackSource)
    candidates := []string{token, src.PlayMedium, urlToHost(src.TrackURI)}
    for _, candidate := range candidates {
        lower := strings.ToLower(candidate)
        for _, known := range knownSources {
            if strings.Contains(lower, known.match) {
                return known.name, "Playing from " + known.name +
                    ". No track list is available for this source."
            }
        }
    }
    for _, raw := range []string{token, src.PlayMedium} {
        raw = strings.TrimSpace(raw)
        if raw != "" && !strings.EqualFold(raw, "NONE") && !strings.EqualFold(raw, "UNKNOWN") {
            return raw, "Playing from " + raw + ". No track list is available for this source."
        }
    }
    return "", ""
}
