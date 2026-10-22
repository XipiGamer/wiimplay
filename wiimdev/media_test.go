package wiimdev

import (
    "encoding/xml"
    "strings"
    "testing"

    "github.com/shumatech/wiimplay/upnp"
)

// Decodifica la cola igual que GetTracks y los metadatos igual que getMetadata
// (copiado de wiim.go), para comprobar que lo que generamos se lee de vuelta.
func TestQueueRoundTrip(t *testing.T) {
    items := []MediaItem{
        {URL: "http://192.168.1.10:8765/t/0.flac", Title: "Rock & Roll <live>", Artist: "AC/DC",
            Album: `"Best" of`, AlbumArt: "http://192.168.1.10:8765/a/0", Mime: "audio/flac"},
        {URL: "http://192.168.1.10:8765/t/1.mp3", Title: "Sin artista", Mime: "audio/mpeg"},
    }
    queue := BuildQueueXML("Mi carpeta", items)

    pl := &upnp.PlayListXml{}
    if err := xml.Unmarshal([]byte(queue), &pl); err != nil {
        t.Fatalf("queue is not valid XML: %v", err)
    }
    if pl.ListName != "Mi carpeta" || pl.ListInfo.TrackNumber != 2 || pl.ListInfo.LastPlayIndex != 1 {
        t.Fatalf("bad list info: %+v", pl)
    }

    reader := strings.NewReader(pl.Tracks.InnerXml)
    decoder := xml.NewDecoder(reader)
    for i := 0; i < pl.ListInfo.TrackNumber; i++ {
        var plt upnp.PlayListTrackXml
        if err := decoder.Decode(&plt); err != nil {
            t.Fatalf("track %d decode: %v", i, err)
        }
        if plt.URL != items[i].URL {
            t.Errorf("track %d url %q", i, plt.URL)
        }
        didl := &upnp.DidlLiteXml{}
        if err := xml.Unmarshal([]byte(plt.Metadata), &didl); err != nil {
            t.Fatalf("track %d metadata: %v", i, err)
        }
        md := didl.Items[0]
        if md.Title != items[i].Title || md.Artist != items[i].Artist ||
            md.Album != items[i].Album || md.AlbumArt != items[i].AlbumArt ||
            md.Res != items[i].URL {
            t.Errorf("track %d metadata mismatch: %+v", i, md)
        }
    }
}

func TestDIDLEscapes(t *testing.T) {
    didl := BuildDIDL("1", MediaItem{URL: "http://x/a?b=1&c=2", Title: "<b>&</b>"})
    if err := xml.Unmarshal([]byte(didl), new(interface{})); err != nil {
        t.Fatalf("invalid xml: %v", err)
    }
    if strings.Contains(didl, "<b>") {
        t.Error("title not escaped")
    }
}
