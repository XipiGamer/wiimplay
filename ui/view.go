package ui

import (
    "context"
    "fmt"
    "strconv"
    "time"

    "github.com/shumatech/wiimplay/ui/res"

    "github.com/diamondburned/gotk4-adwaita/pkg/adw"
    "github.com/diamondburned/gotk4/pkg/gdk/v4"
    "github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
    "github.com/diamondburned/gotk4/pkg/gio/v2"
    "github.com/diamondburned/gotk4/pkg/glib/v2"
    "github.com/diamondburned/gotk4/pkg/gtk/v4"
    "github.com/diamondburned/gotk4/pkg/pango"
)

type ViewListener interface {
    SeekSelect(position float64)
    ShufflePress()
    PreviousPress()
    PlayPausePress()
    NextPress()
    LoopPress()
    VolumeSelect(level int)
    PlayListSelect(index int)
    TrackSelect(index int)
    WindowHidden(flag bool)
    MutePress()
    SettingsPress()
    DeviceSelect(index int)
    RefreshPress()
    AboutPress()
    EqualizerSelect(name string)
    ControlsPress()
    FolderSelect(path string)
    LocalTrackSelect(index int)
}

// GTK_INVALID_LIST_POSITION: valor de GtkDropDown cuando no hay selección.
const invalidPosition uint = 0xFFFFFFFF

type View struct {
    listener ViewListener

    application *gtk.Application
    window *adw.ApplicationWindow
    hideOnClose bool
    lastFolder string

    devicesDrop *gtk.DropDown
    devicesModel *gtk.StringList
    updatingDevices bool

    albumPicture *gtk.Picture

    titleLabel *gtk.Label
    artistLabel *gtk.Label
    albumLabel *gtk.Label
    infoLabel *gtk.Label

    timeUsed *gtk.Label
    timeTotal *gtk.Label
    seekAdjust *gtk.Adjustment
    seekScale *gtk.Scale
    seekTimer glib.SourceHandle
    seekHold time.Time

    shuffleButton *gtk.Button
    previousButton *gtk.Button
    playButton *gtk.Button
    playImage *gtk.Image
    nextButton *gtk.Button
    loopButton *gtk.Button

    muteButton *gtk.Button
    volumeAdjust *gtk.Adjustment
    volumeScale *gtk.Scale
    updatingVolume bool
    eqButton *gtk.MenuButton
    eqPopover *gtk.Popover

    contentStack *adw.ViewStack
    wiimStack *gtk.Stack
    sourcePage *adw.StatusPage
    localStack *gtk.Stack
    localList *gtk.ListBox
    localFolderLabel *gtk.Label

    playListBox *gtk.ListBox
    trackThumbs []*gtk.Picture
    playListDrop *gtk.DropDown
    playListModel *gtk.StringList
    updatingPlayList bool

    defaultAlbum *gdk.Texture
    defaultTrack *gdk.Texture
}

const (
    AlbumImageWidth = 256
    AlbumImageHeight = 256
    TrackImageWidth = 48
    TrackImageHeight = 48

    VolumePageLevel = 5

    // Tiempo que se ignoran las actualizaciones de posición del dispositivo
    // tras arrastrar la barra de búsqueda, para que no "salte" hacia atrás.
    seekHoldTime = 1500 * time.Millisecond
    seekDebounce = 250
)

///////////////////////////////////////////////////////////////////////////////
// Helpers de construcción
///////////////////////////////////////////////////////////////////////////////

func newIconButton(icon string, tooltip string, classes ...string) *gtk.Button {
    button := gtk.NewButtonFromIconName(icon)
    button.SetTooltipText(tooltip)
    for _, class := range classes {
        button.AddCSSClass(class)
    }
    return button
}

func newCenteredLabel(classes ...string) *gtk.Label {
    label := gtk.NewLabel("")
    label.SetJustify(gtk.JustifyCenter)
    label.SetWrap(true)
    label.SetWrapMode(pango.WrapWordChar)
    label.SetMaxWidthChars(32)
    for _, class := range classes {
        label.AddCSSClass(class)
    }
    return label
}

func newDropDown(model *gtk.StringList) *gtk.DropDown {
    drop := gtk.NewDropDown(model, nil)
    drop.SetEnableSearch(false)
    return drop
}

func clearStringList(model *gtk.StringList) {
    model.Splice(0, model.NItems(), nil)
}

///////////////////////////////////////////////////////////////////////////////
// Construcción de la interfaz
///////////////////////////////////////////////////////////////////////////////

func (view *View) addActions() {
    settings := gio.NewSimpleAction("settings", nil)
    settings.ConnectActivate(func(_ *glib.Variant) {
        view.listener.SettingsPress()
    })
    view.application.AddAction(settings)

    about := gio.NewSimpleAction("about", nil)
    about.ConnectActivate(func(_ *glib.Variant) {
        view.listener.AboutPress()
    })
    view.application.AddAction(about)

    folder := gio.NewSimpleAction("folder", nil)
    folder.ConnectActivate(func(_ *glib.Variant) {
        view.chooseFolder()
    })
    view.application.AddAction(folder)
}

// Diálogo nativo de GTK4 (portal de GNOME) para elegir la carpeta de música.
func (view *View) chooseFolder() {
    dialog := gtk.NewFileDialog()
    dialog.SetTitle("Choose a music folder")
    dialog.SetModal(true)
    if view.lastFolder != "" {
        dialog.SetInitialFolder(gio.NewFileForPath(view.lastFolder))
    }
    dialog.SelectFolder(context.Background(), &view.window.Window,
        func(res gio.AsyncResulter) {
            file, err := dialog.SelectFolderFinish(res)
            if err != nil || file == nil {
                return // cancelado
            }
            path := file.Path()
            if path == "" {
                return
            }
            view.lastFolder = path
            view.listener.FolderSelect(path)
        })
}

func (view *View) SetLastFolder(path string) {
    view.lastFolder = path
}

func (view *View) newPrimaryMenu() *gtk.MenuButton {
    menu := gio.NewMenu()
    menu.Append("Play local folder…", "app.folder")
    menu.Append("Settings", "app.settings")
    menu.Append("About WiiM Play", "app.about")

    button := gtk.NewMenuButton()
    button.SetIconName("open-menu-symbolic")
    button.SetTooltipText("Main menu")
    button.SetMenuModel(menu)
    return button
}

// Barra superior del panel del reproductor: selector de dispositivo.
func (view *View) newPlayerHeader() *adw.HeaderBar {
    header := adw.NewHeaderBar()
    header.SetShowEndTitleButtons(false)

    view.devicesModel = gtk.NewStringList(nil)
    view.devicesDrop = newDropDown(view.devicesModel)
    view.devicesDrop.SetTooltipText("Device")
    view.devicesDrop.Connect("notify::selected", func() {
        if view.updatingDevices {
            return
        }
        index := view.devicesDrop.Selected()
        if index != invalidPosition {
            view.listener.DeviceSelect(int(index))
        }
    })
    header.SetTitleWidget(view.devicesDrop)

    refresh := newIconButton("view-refresh-symbolic", "Search for devices")
    refresh.ConnectClicked(func() { view.listener.RefreshPress() })
    header.PackStart(refresh)

    controls := newIconButton("preferences-system-symbolic", "Device controls")
    controls.ConnectClicked(func() { view.listener.ControlsPress() })
    header.PackEnd(controls)

    return header
}

// Barra superior del panel de contenido: selector de origen (WiiM / carpeta
// local) y menú principal.
func (view *View) newQueueHeader() *adw.HeaderBar {
    header := adw.NewHeaderBar()
    header.SetShowStartTitleButtons(false)

    switcher := adw.NewViewSwitcher()
    switcher.SetStack(view.contentStack)
    switcher.SetPolicy(adw.ViewSwitcherPolicyWide)
    header.SetTitleWidget(switcher)

    header.PackEnd(view.newPrimaryMenu())
    return header
}

func (view *View) newSeekBar() *gtk.Box {
    box := gtk.NewBox(gtk.OrientationVertical, 0)

    view.seekAdjust = gtk.NewAdjustment(0, 0, 1, 0.01, 0.05, 0)
    view.seekScale = gtk.NewScale(gtk.OrientationHorizontal, view.seekAdjust)
    view.seekScale.SetDrawValue(false)
    view.seekScale.SetHExpand(true)
    // "change-value" solo se emite por interacción del usuario, no cuando
    // la posición se actualiza desde el dispositivo.
    view.seekScale.ConnectChangeValue(func(scroll gtk.ScrollType, value float64) bool {
        if value < 0 {
            value = 0
        } else if value > 1 {
            value = 1
        }
        view.seekHold = time.Now().Add(seekHoldTime)
        if view.seekTimer != 0 {
            glib.SourceRemove(view.seekTimer)
        }
        view.seekTimer = glib.TimeoutAdd(seekDebounce, func() bool {
            view.seekTimer = 0
            view.listener.SeekSelect(value)
            return false
        })
        return false
    })
    box.Append(view.seekScale)

    times := gtk.NewBox(gtk.OrientationHorizontal, 0)
    view.timeUsed = gtk.NewLabel("0:00")
    view.timeUsed.AddCSSClass("caption")
    view.timeUsed.AddCSSClass("dim-label")
    view.timeUsed.SetHExpand(true)
    view.timeUsed.SetXAlign(0)
    times.Append(view.timeUsed)

    view.timeTotal = gtk.NewLabel("0:00")
    view.timeTotal.AddCSSClass("caption")
    view.timeTotal.AddCSSClass("dim-label")
    view.timeTotal.SetXAlign(1)
    times.Append(view.timeTotal)
    box.Append(times)

    return box
}

func (view *View) newTransportControls() *gtk.Box {
    box := gtk.NewBox(gtk.OrientationHorizontal, 12)
    box.SetHAlign(gtk.AlignCenter)

    view.shuffleButton = newIconButton("media-playlist-shuffle-symbolic",
        "Shuffle", "flat", "circular")
    view.shuffleButton.ConnectClicked(func() { view.listener.ShufflePress() })
    box.Append(view.shuffleButton)

    view.previousButton = newIconButton("media-skip-backward-symbolic",
        "Previous", "flat", "circular")
    view.previousButton.ConnectClicked(func() { view.listener.PreviousPress() })
    box.Append(view.previousButton)

    view.playImage = gtk.NewImageFromIconName("media-playback-start-symbolic")
    view.playImage.SetPixelSize(24)
    view.playButton = gtk.NewButton()
    view.playButton.SetChild(view.playImage)
    view.playButton.SetTooltipText("Play/Pause")
    view.playButton.AddCSSClass("circular")
    view.playButton.AddCSSClass("suggested-action")
    view.playButton.SetSizeRequest(56, 56)
    view.playButton.ConnectClicked(func() { view.listener.PlayPausePress() })
    box.Append(view.playButton)

    view.nextButton = newIconButton("media-skip-forward-symbolic",
        "Next", "flat", "circular")
    view.nextButton.ConnectClicked(func() { view.listener.NextPress() })
    box.Append(view.nextButton)

    view.loopButton = newIconButton("media-playlist-repeat-symbolic",
        "Repeat", "flat", "circular")
    view.loopButton.ConnectClicked(func() { view.listener.LoopPress() })
    box.Append(view.loopButton)

    return box
}

func (view *View) newVolumeBar() *gtk.Box {
    box := gtk.NewBox(gtk.OrientationHorizontal, 6)

    view.muteButton = newIconButton("audio-volume-high-symbolic",
        "Mute", "flat", "circular")
    view.muteButton.ConnectClicked(func() { view.listener.MutePress() })
    box.Append(view.muteButton)

    view.volumeAdjust = gtk.NewAdjustment(0, 0, 100, 1, VolumePageLevel, 0)
    view.volumeAdjust.ConnectValueChanged(func() {
        if view.updatingVolume {
            return
        }
        view.listener.VolumeSelect(int(view.volumeAdjust.Value() + 0.5))
    })
    view.volumeScale = gtk.NewScale(gtk.OrientationHorizontal, view.volumeAdjust)
    view.volumeScale.SetDrawValue(false)
    view.volumeScale.SetHExpand(true)
    box.Append(view.volumeScale)

    // El popover del ecualizador se rellena en SetEqualizers
    view.eqPopover = gtk.NewPopover()
    view.eqButton = gtk.NewMenuButton()
    view.eqButton.SetIconName("media-eq-symbolic")
    view.eqButton.SetTooltipText("Equalizer")
    view.eqButton.AddCSSClass("flat")
    view.eqButton.AddCSSClass("circular")
    view.eqButton.SetPopover(view.eqPopover)
    view.eqButton.SetSensitive(false)
    box.Append(view.eqButton)

    return box
}

func (view *View) newPlayerPage() gtk.Widgetter {
    box := gtk.NewBox(gtk.OrientationVertical, 12)
    box.SetMarginTop(18)
    box.SetMarginBottom(18)
    box.SetMarginStart(18)
    box.SetMarginEnd(18)

    view.albumPicture = gtk.NewPictureForPaintable(view.defaultAlbum)
    view.albumPicture.SetContentFit(gtk.ContentFitCover)
    view.albumPicture.SetCanShrink(true)
    view.albumPicture.SetSizeRequest(AlbumImageWidth, AlbumImageHeight)
    view.albumPicture.SetHAlign(gtk.AlignCenter)
    view.albumPicture.SetVExpand(false)
    view.albumPicture.AddCSSClass("card")
    box.Append(view.albumPicture)

    view.titleLabel = newCenteredLabel("title-3")
    view.titleLabel.SetMarginTop(6)
    box.Append(view.titleLabel)

    view.artistLabel = newCenteredLabel()
    box.Append(view.artistLabel)

    view.albumLabel = newCenteredLabel("dim-label")
    box.Append(view.albumLabel)

    view.infoLabel = newCenteredLabel("caption", "dim-label")
    box.Append(view.infoLabel)

    spacer := gtk.NewBox(gtk.OrientationVertical, 0)
    spacer.SetVExpand(true)
    box.Append(spacer)

    box.Append(view.newSeekBar())
    box.Append(view.newTransportControls())
    box.Append(view.newVolumeBar())

    return box
}

func newBoxedList() *gtk.ListBox {
    list := gtk.NewListBox()
    list.SetSelectionMode(gtk.SelectionSingle)
    list.AddCSSClass("boxed-list")
    list.SetMarginTop(6)
    list.SetMarginBottom(12)
    list.SetMarginStart(12)
    list.SetMarginEnd(12)
    list.SetVAlign(gtk.AlignStart)
    return list
}

func newListScroll(list *gtk.ListBox) *gtk.ScrolledWindow {
    scroll := gtk.NewScrolledWindow()
    scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
    scroll.SetVExpand(true)
    scroll.SetChild(list)
    return scroll
}

func newStatusPage(icon string, title string, description string) *adw.StatusPage {
    page := adw.NewStatusPage()
    page.SetIconName(icon)
    page.SetTitle(title)
    page.SetDescription(description)
    page.SetVExpand(true)
    return page
}

// Página "WiiM": playlist del dispositivo y su cola. Si el WiiM no reproduce
// desde una playlist se muestra una explicación en lugar de un panel vacío.
func (view *View) newWiimPage() gtk.Widgetter {
    box := gtk.NewBox(gtk.OrientationVertical, 0)

    view.playListModel = gtk.NewStringList(nil)
    view.playListDrop = newDropDown(view.playListModel)
    view.playListDrop.SetTooltipText("Playlist")
    view.playListDrop.SetHExpand(true)
    view.playListDrop.Connect("notify::selected", func() {
        if view.updatingPlayList {
            return
        }
        index := view.playListDrop.Selected()
        if index != invalidPosition {
            view.listener.PlayListSelect(int(index))
        }
    })
    bar := gtk.NewBox(gtk.OrientationHorizontal, 6)
    bar.SetMarginTop(12)
    bar.SetMarginStart(12)
    bar.SetMarginEnd(12)
    bar.Append(view.playListDrop)
    // El selector de playlists del WiiM se mantiene (el controlador lo
    // rellena) pero oculto: la interfaz no lo necesita por ahora.
    bar.SetVisible(false)
    box.Append(bar)

    view.playListBox = newBoxedList()
    view.playListBox.ConnectRowActivated(func(row *gtk.ListBoxRow) {
        if row != nil {
            view.listener.TrackSelect(row.Index())
        }
    })

    view.wiimStack = gtk.NewStack()
    view.wiimStack.SetVExpand(true)
    view.wiimStack.AddNamed(newListScroll(view.playListBox), "list")
    view.sourcePage = newStatusPage("audio-x-generic-symbolic", "", "")
    view.sourcePage.AddCSSClass("compact")
    view.wiimStack.AddNamed(view.sourcePage, "empty")
    view.SetSource("", "")
    view.wiimStack.SetVisibleChildName("empty")
    box.Append(view.wiimStack)
    return box
}

// Página "Local folder": carpeta elegida y su lista de pistas.
func (view *View) newLocalPage() gtk.Widgetter {
    box := gtk.NewBox(gtk.OrientationVertical, 0)

    view.localFolderLabel = gtk.NewLabel("No folder selected")
    view.localFolderLabel.AddCSSClass("dim-label")
    view.localFolderLabel.SetXAlign(0)
    view.localFolderLabel.SetHExpand(true)
    view.localFolderLabel.SetEllipsize(pango.EllipsizeStart)

    choose := gtk.NewButtonWithLabel("Choose folder…")
    choose.ConnectClicked(func() { view.chooseFolder() })

    bar := gtk.NewBox(gtk.OrientationHorizontal, 6)
    bar.SetMarginTop(12)
    bar.SetMarginStart(12)
    bar.SetMarginEnd(12)
    bar.Append(view.localFolderLabel)
    bar.Append(choose)
    box.Append(bar)

    view.localList = newBoxedList()
    view.localList.ConnectRowActivated(func(row *gtk.ListBoxRow) {
        if row != nil {
            view.listener.LocalTrackSelect(row.Index())
        }
    })

    emptyButton := gtk.NewButtonWithLabel("Choose folder…")
    emptyButton.AddCSSClass("suggested-action")
    emptyButton.AddCSSClass("pill")
    emptyButton.SetHAlign(gtk.AlignCenter)
    emptyButton.ConnectClicked(func() { view.chooseFolder() })
    empty := newStatusPage("folder-music-symbolic", "Play music from this computer",
        "Choose a folder and the WiiM will stream it from this computer "+
        "over your network.")
    empty.SetChild(emptyButton)

    view.localStack = gtk.NewStack()
    view.localStack.SetVExpand(true)
    view.localStack.AddNamed(newListScroll(view.localList), "list")
    view.localStack.AddNamed(empty, "empty")
    view.localStack.SetVisibleChildName("empty")
    box.Append(view.localStack)
    return box
}

// Contenedor con el selector de origen. El WiiM es siempre el origen por
// defecto; la carpeta local solo se muestra al elegirla.
func (view *View) newContentStack() *adw.ViewStack {
    stack := adw.NewViewStack()
    stack.AddTitledWithIcon(view.newWiimPage(), "wiim", "WiiM", "audio-speakers-symbolic")
    stack.AddTitledWithIcon(view.newLocalPage(), "local", "Local folder", "folder-music-symbolic")
    stack.SetVisibleChildName("wiim")
    return stack
}

func newToolbarPage(header *adw.HeaderBar, content gtk.Widgetter,
                    title string) *adw.NavigationPage {
    toolbar := adw.NewToolbarView()
    toolbar.AddTopBar(header)
    toolbar.SetContent(content)
    return adw.NewNavigationPage(toolbar, title)
}

func formatInfo(quality string, rate int, format string, bitrate int) string {
    str := ""
    sep := ""
    if quality != "" {
        str += quality
        sep = " "
    }
    if rate != 0 {
        if rate % 1000 == 0 {
            str += sep + strconv.Itoa(rate / 1000) + " kHz"
        } else if rate % 100 == 0 {
            str += fmt.Sprintf("%s%.1f kHz", sep, float32(rate) / 1000)
        } else {
            str += sep + strconv.Itoa(rate) + " Hz"
        }
        sep = ", "
    }
    if format != "" {
        str += sep + format + " bit"
        sep = ", "
    }
    if bitrate != 0 {
        str += sep + strconv.Itoa(bitrate) + " kbps"
    }
    return str
}

func formatDuration(dur time.Duration) string {
    h := int(dur.Hours())
    m := int(dur.Minutes()) % 60
    s := int(dur.Seconds()) % 60
    if h > 0 {
        return fmt.Sprintf("%d:%02d:%02d", h, m, s)
    } else {
        return fmt.Sprintf("%d:%02d", m, s)
    }
}

///////////////////////////////////////////////////////////////////////////////
// Public interface
///////////////////////////////////////////////////////////////////////////////

func NewView(application *gtk.Application, listener ViewListener) (*View, error) {
    view := &View{}
    view.application = application
    view.listener = listener

    var err error
    view.defaultAlbum, err = textureFromData(res.DefaultAlbum)
    if err != nil {
        return nil, err
    }
    view.defaultTrack, err = textureFromData(res.DefaultTrack)
    if err != nil {
        return nil, err
    }

    view.addActions()

    window := adw.NewApplicationWindow(application)
    view.window = window
    window.SetTitle("WiiM Play")
    window.SetDefaultSize(940, 660)

    // Mantiene el comportamiento "Hide on close": cerrar solo oculta la ventana.
    window.ConnectCloseRequest(func() bool {
        if view.hideOnClose {
            window.SetVisible(false)
            return true
        }
        return false
    })
    window.ConnectShow(func() { view.listener.WindowHidden(false) })
    window.ConnectHide(func() { view.listener.WindowHidden(true) })

    playerPage := newToolbarPage(view.newPlayerHeader(), view.newPlayerPage(), "WiiM Play")
    view.contentStack = view.newContentStack()
    queuePage := newToolbarPage(view.newQueueHeader(), view.contentStack, "Queue")

    split := adw.NewNavigationSplitView()
    split.SetMinSidebarWidth(340)
    split.SetMaxSidebarWidth(460)
    split.SetSidebarWidthFraction(0.4)
    split.SetSidebar(playerPage)
    split.SetContent(queuePage)

    window.SetContent(split)
    window.Present()

    return view, nil
}

type PlayerState uint
const (
    PlayerUnavailable PlayerState = iota
    PlayerPlaying
    PlayerPaused
)

func (view *View) SetPlayerState(state PlayerState) {
    icon := "media-playback-start-symbolic"
    if state == PlayerPlaying {
        icon = "media-playback-pause-symbolic"
    }
    view.playImage.SetFromIconName(icon)
}

func (view *View) SetInfo(quality string, rate int, format string, bitrate int) {
    view.infoLabel.SetText(formatInfo(quality, rate, format, bitrate))
}

func (view *View) SetPlaying(title string, artist string, album string) {
    view.titleLabel.SetText(title)
    view.artistLabel.SetText(artist)
    view.albumLabel.SetText(album)
}

func (view *View) SetStatus(status string) {
    view.SetPlaying(status, "", "")
}

func (view *View) SetAlbumArt(album *gdkpixbuf.Pixbuf) {
    if album == nil {
        view.albumPicture.SetPaintable(view.defaultAlbum)
        return
    }
    view.albumPicture.SetPaintable(gdk.NewTextureForPixbuf(album))
}

func (view *View) SetSeek(used time.Duration, total time.Duration) {
    position := 0.0
    if total != time.Duration(0) {
        position = used.Seconds() / total.Seconds()
    }
    view.timeUsed.SetText(formatDuration(used))
    view.timeTotal.SetText(formatDuration(total))
    if time.Now().After(view.seekHold) {
        view.seekAdjust.SetValue(position)
    }
}

func (view *View) SetVolume(level int) {
    view.updatingVolume = true
    view.volumeAdjust.SetValue(float64(level))
    view.updatingVolume = false
}

func (view *View) SetMute(mute bool) {
    if mute {
        view.muteButton.SetIconName("audio-volume-muted-symbolic")
    } else {
        view.muteButton.SetIconName("audio-volume-high-symbolic")
    }
    view.volumeScale.SetSensitive(!mute)
}

func setToggled(button *gtk.Button, on bool) {
    if on {
        button.AddCSSClass("accent")
    } else {
        button.RemoveCSSClass("accent")
    }
}

func (view *View) SetShuffle(on bool) {
    setToggled(view.shuffleButton, on)
}

type LoopState uint
const (
    LoopNone LoopState = iota
    LoopOne
    LoopAll
)

func (view *View) SetLoopState(state LoopState) {
    if state == LoopOne {
        view.loopButton.SetIconName("media-playlist-repeat-song-symbolic")
    } else {
        view.loopButton.SetIconName("media-playlist-repeat-symbolic")
    }
    setToggled(view.loopButton, state != LoopNone)
}

func (view *View) SetPlayList(index int) {
    view.updatingPlayList = true
    if index < 0 {
        view.playListDrop.SetSelected(invalidPosition)
    } else {
        view.playListDrop.SetSelected(uint(index))
    }
    view.updatingPlayList = false
}

func (view *View) AddPlayList(name string) {
    view.updatingPlayList = true
    view.playListModel.Append(name)
    view.updatingPlayList = false
}

func (view *View) ClearPlayLists() {
    view.updatingPlayList = true
    clearStringList(view.playListModel)
    view.updatingPlayList = false
}

func (view *View) SetTrack(index int) {
    row := view.playListBox.RowAtIndex(index)
    if row == nil {
        view.playListBox.UnselectAll()
        return
    }
    view.playListBox.SelectRow(row)
    glib.TimeoutAdd(100, func() bool {
        if row := view.playListBox.RowAtIndex(index); row != nil {
            row.GrabFocus()
        }
        return false
    })
}

func (view *View) AddTrack(title string, artist string) error {
    row := adw.NewActionRow()
    row.SetTitle(Sanitize(title))
    row.SetSubtitle(Sanitize(artist))
    row.SetActivatable(true)

    thumb := gtk.NewPictureForPaintable(view.defaultTrack)
    thumb.SetContentFit(gtk.ContentFitCover)
    thumb.SetCanShrink(true)
    thumb.SetSizeRequest(TrackImageWidth, TrackImageHeight)
    thumb.SetVAlign(gtk.AlignCenter)
    thumb.AddCSSClass("card")
    row.AddPrefix(thumb)

    view.trackThumbs = append(view.trackThumbs, thumb)
    view.playListBox.Append(row)
    view.wiimStack.SetVisibleChildName("list")
    return nil
}

func (view *View) SetTrackAlbumArt(index int, album *gdkpixbuf.Pixbuf) error {
    if index < 0 || index >= len(view.trackThumbs) {
        return nil
    }
    if album == nil {
        view.trackThumbs[index].SetPaintable(view.defaultTrack)
    } else {
        view.trackThumbs[index].SetPaintable(gdk.NewTextureForPixbuf(album))
    }
    return nil
}

func (view *View) ClearTracks() {
    view.playListBox.RemoveAll()
    view.trackThumbs = nil
    view.wiimStack.SetVisibleChildName("empty")
}

func (view *View) GetWindow() gtk.Widgetter {
    return view.window
}

func (view *View) Show() {
    view.window.SetVisible(true)
    view.window.Present()
}

func (view *View) Hide() {
    view.window.SetVisible(false)
}

func (view *View) HideOnDelete(enable bool) {
    view.hideOnClose = enable
}

func (view *View) AddDevice(name string) {
    view.updatingDevices = true
    view.devicesModel.Append(name)
    view.updatingDevices = false
}

func (view *View) SetDevice(index int) {
    view.updatingDevices = true
    if index < 0 {
        view.devicesDrop.SetSelected(invalidPosition)
    } else {
        view.devicesDrop.SetSelected(uint(index))
    }
    view.updatingDevices = false
}

func (view *View) ClearDevices() {
    view.updatingDevices = true
    clearStringList(view.devicesModel)
    view.updatingDevices = false
}

func (view *View) ClearEqualizers() {
    view.eqPopover.SetChild(gtk.NewBox(gtk.OrientationVertical, 0))
    view.eqButton.SetSensitive(false)
    view.eqButton.RemoveCSSClass("accent")
}

func (view *View) SetEqualizers(names []string, selected string) error {
    view.ClearEqualizers()

    box := gtk.NewBox(gtk.OrientationVertical, 2)
    box.SetMarginTop(6)
    box.SetMarginBottom(6)
    box.SetMarginStart(6)
    box.SetMarginEnd(6)

    var first *gtk.CheckButton
    for _, name := range names {
        name := name
        item := gtk.NewCheckButtonWithLabel(name)
        if first == nil {
            first = item
        } else {
            item.SetGroup(first)
        }
        // Se activa antes de conectar la señal para no disparar EqualizerSelect.
        if name == selected {
            item.SetActive(true)
        }
        item.ConnectToggled(func() {
            if item.Active() {
                view.listener.EqualizerSelect(name)
            }
        })
        box.Append(item)
    }

    view.eqPopover.SetChild(box)
    view.eqButton.SetSensitive(len(names) > 0)
    return nil
}

func (view *View) SetEqualizerOn(on bool) {
    if on {
        view.eqButton.AddCSSClass("accent")
    } else {
        view.eqButton.RemoveCSSClass("accent")
    }
}

// Entrada de la lista de la carpeta local.
type LocalEntry struct {
    Title    string
    Subtitle string
}

func (view *View) ShowLocal() {
    view.contentStack.SetVisibleChildName("local")
}

func (view *View) ShowWiim() {
    view.contentStack.SetVisibleChildName("wiim")
}

func (view *View) SetLocalFolder(path string) {
    view.localFolderLabel.SetText(path)
    view.localFolderLabel.SetTooltipText(path)
}

func (view *View) SetLocalTracks(entries []LocalEntry) {
    view.localList.RemoveAll()
    for i, entry := range entries {
        row := adw.NewActionRow()
        row.SetTitle(Sanitize(entry.Title))
        row.SetSubtitle(Sanitize(entry.Subtitle))
        row.SetActivatable(true)

        number := gtk.NewLabel(strconv.Itoa(i + 1))
        number.AddCSSClass("dim-label")
        number.AddCSSClass("numeric")
        number.SetWidthChars(3)
        number.SetXAlign(1)
        row.AddPrefix(number)

        view.localList.Append(row)
    }
    if len(entries) > 0 {
        view.localStack.SetVisibleChildName("list")
    } else {
        view.localStack.SetVisibleChildName("empty")
    }
}

// Marca la pista local que está sonando (-1 para ninguna).
func (view *View) SetLocalTrack(index int) {
    row := view.localList.RowAtIndex(index)
    if index < 0 || row == nil {
        view.localList.UnselectAll()
        return
    }
    view.localList.SelectRow(row)
}

// Panel que se muestra cuando el WiiM no reproduce desde una cola: indica la
// fuente (Spotify, TIDAL, ...) si el dispositivo la informa.
func (view *View) SetSource(name string, description string) {
    if name == "" {
        view.sourcePage.SetTitle("Nothing queued")
        view.sourcePage.SetDescription("The WiiM is not playing from a playlist. " +
            "Play a folder from this computer in the Local folder tab.")
        return
    }
    view.sourcePage.SetTitle(name)
    view.sourcePage.SetDescription(description)
}

// Fila de las listas de álbumes y canciones del artista.
type InfoRow struct {
    Title    string
    Subtitle string
}

func newInfoGroup(title string, rows []InfoRow, numbered bool) *adw.PreferencesGroup {
    group := adw.NewPreferencesGroup()
    group.SetTitle(title)
    for i, r := range rows {
        row := adw.NewActionRow()
        row.SetTitle(Sanitize(r.Title))
        row.SetSubtitle(Sanitize(r.Subtitle))
        if numbered {
            number := gtk.NewLabel(strconv.Itoa(i + 1))
            number.AddCSSClass("dim-label")
            number.AddCSSClass("numeric")
            number.SetWidthChars(2)
            number.SetXAlign(1)
            row.AddPrefix(number)
        }
        group.Add(row)
    }
    return group
}

// Muestra bajo la fuente los álbumes y canciones más populares del artista
// (sin datos: no se muestra nada).
func (view *View) SetArtistInfo(artist string, albums []InfoRow, tracks []InfoRow) {
    box := gtk.NewBox(gtk.OrientationVertical, 18)
    box.SetMarginTop(6)
    box.SetMarginBottom(12)
    box.SetMarginStart(12)
    box.SetMarginEnd(12)
    if len(albums) == 0 && len(tracks) == 0 {
        view.sourcePage.SetChild(box)
        return
    }

    heading := newCenteredLabel("title-3")
    heading.SetText(artist)
    box.Append(heading)
    if len(albums) > 0 {
        box.Append(newInfoGroup("Most popular albums", albums, false))
    }
    if len(tracks) > 0 {
        box.Append(newInfoGroup("Top songs", tracks, true))
    }
    note := newCenteredLabel("caption", "dim-label")
    note.SetText("Popularity by streams on Deezer, not sales figures.")
    box.Append(note)

    clamp := adw.NewClamp()
    clamp.SetMaximumSize(520)
    clamp.SetChild(box)
    view.sourcePage.SetChild(clamp)
}

// Mensaje bajo la fuente cuando no se pudo mostrar información del artista
// (no encontrado en Deezer, sin conexión...). Texto vacío: no muestra nada.
func (view *View) SetArtistNote(text string) {
    box := gtk.NewBox(gtk.OrientationVertical, 0)
    box.SetMarginTop(6)
    box.SetMarginBottom(12)
    if text != "" {
        note := newCenteredLabel("caption", "dim-label")
        note.SetText(text)
        box.Append(note)
    }
    view.sourcePage.SetChild(box)
}
