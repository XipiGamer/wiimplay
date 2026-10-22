package wiimdev

import (
    "bytes"
    "encoding/xml"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

// MediaItem describe una pista que el WiiM debe reproducir desde una URL
// (por ejemplo, servida por el módulo localmedia).
type MediaItem struct {
    URL      string
    Title    string
    Artist   string
    Album    string
    AlbumArt string
    Mime     string
}

func xmlEscape(s string) string {
    var buf bytes.Buffer
    xml.EscapeText(&buf, []byte(s))
    return buf.String()
}

// BuildDIDL genera los metadatos DIDL-Lite de una pista.
func BuildDIDL(id string, item MediaItem) string {
    mime := item.Mime
    if mime == "" {
        mime = "*"
    }
    var b strings.Builder
    b.WriteString(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" ` +
        `xmlns:dc="http://purl.org/dc/elements/1.1/" ` +
        `xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">`)
    fmt.Fprintf(&b, `<item id="%s" parentID="0" restricted="1">`, xmlEscape(id))
    fmt.Fprintf(&b, `<dc:title>%s</dc:title>`, xmlEscape(item.Title))
    if item.Artist != "" {
        fmt.Fprintf(&b, `<upnp:artist>%s</upnp:artist><dc:creator>%s</dc:creator>`,
            xmlEscape(item.Artist), xmlEscape(item.Artist))
    }
    if item.Album != "" {
        fmt.Fprintf(&b, `<upnp:album>%s</upnp:album>`, xmlEscape(item.Album))
    }
    if item.AlbumArt != "" {
        fmt.Fprintf(&b, `<upnp:albumArtURI>%s</upnp:albumArtURI>`, xmlEscape(item.AlbumArt))
    }
    b.WriteString(`<upnp:class>object.item.audioItem.musicTrack</upnp:class>`)
    fmt.Fprintf(&b, `<res protocolInfo="http-get:*:%s:*">%s</res>`,
        xmlEscape(mime), xmlEscape(item.URL))
    b.WriteString(`</item></DIDL-Lite>`)
    return b.String()
}

const localSource = "LocalMedia"

// BuildQueueXML genera el QueueContext de la acción PlayQueue.CreateQueue con
// la estructura que el WiiM devuelve en BrowseQueue (ListName, ListInfo y
// Track1..TrackN). Los índices del WiiM empiezan en 1.
func BuildQueueXML(name string, items []MediaItem) string {
    var b strings.Builder
    b.WriteString(`<PlayList>`)
    fmt.Fprintf(&b, `<ListName>%s</ListName>`, xmlEscape(name))
    b.WriteString(`<ListInfo>`)
    fmt.Fprintf(&b, `<SourceName>%s</SourceName>`, localSource)
    fmt.Fprintf(&b, `<TrackNumber>%d</TrackNumber>`, len(items))
    b.WriteString(`<Quality>0</Quality><LastPlayIndex>1</LastPlayIndex>`)
    b.WriteString(`</ListInfo><Tracks>`)
    for i, item := range items {
        n := i + 1
        fmt.Fprintf(&b, `<Track%d><URL>%s</URL><Metadata>%s</Metadata><Id>%d</Id><Source>%s</Source></Track%d>`,
            n, xmlEscape(item.URL), xmlEscape(BuildDIDL(fmt.Sprint(n), item)),
            n, localSource, n)
    }
    b.WriteString(`</Tracks></PlayList>`)
    return b.String()
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Con WIIMPLAY_DEBUG=1 guarda en el directorio temporal el XML intercambiado
// con el dispositivo (colas enviadas y recibidas) para poder inspeccionarlo.
func debugDump(name string, data string) {
    if os.Getenv("WIIMPLAY_DEBUG") == "" {
        return
    }
    path := filepath.Join(os.TempDir(), "wiimplay-"+unsafeName.ReplaceAllString(name, "_")+".xml")
    if err := os.WriteFile(path, []byte(data), 0644); err != nil {
        log.Printf("debug dump failed: %v", err)
        return
    }
    log.Printf("Debug: wrote %s (%d bytes)", path, len(data))
}
