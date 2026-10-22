package artistinfo

import (
    "context"
    "errors"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
)

type fake struct {
    t       *testing.T
    handler map[string]string // ruta -> JSON
    status  map[string]int
    queries []url.Values
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    f.queries = append(f.queries, r.URL.Query())
    if code, ok := f.status[r.URL.Path]; ok {
        w.WriteHeader(code)
        return
    }
    body, ok := f.handler[r.URL.Path]
    if !ok {
        http.NotFound(w, r)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(body))
}

func newClient(t *testing.T, f *fake) (*Client, func()) {
    srv := httptest.NewServer(f)
    c := NewClient()
    c.base = srv.URL
    return c, srv.Close
}

const artistSearch = `{"data":[
 {"id":1,"name":"Tate McRae Tribute Band","nb_fan":5},
 {"id":42,"name":"tate mcrae","nb_fan":9000000},
 {"id":43,"name":"Tate McRae","nb_fan":12}
]}`

const topTracks = `{"data":[
 {"title":"greedy","title_short":"greedy","album":{"title":"THINK LATER"}},
 {"title":"greedy (Remix)","title_short":"greedy","album":{"title":"greedy (Remix)"}},
 {"title":"exes","album":{"title":"I used to think I could fly"}},
 {"title":"Greedy","album":{"title":"THINK LATER"}},
 {"title":"","title_short":"2 hands","album":{"title":"THINK LATER"}}
]}`

const albumSearch = `{"data":[
 {"id":1,"title":"THINK LATER (Deluxe)","nb_tracks":20,"record_type":"album","artist":{"id":42,"name":"Tate McRae"}},
 {"id":2,"title":"Greatest Hits of Others","nb_tracks":10,"record_type":"album","artist":{"id":99,"name":"Someone Else"}},
 {"id":3,"title":"THINK LATER","nb_tracks":14,"record_type":"album","artist":{"id":42,"name":"Tate McRae"}},
 {"id":4,"title":"greedy","nb_tracks":1,"record_type":"single","artist":{"id":42,"name":"Tate McRae"}},
 {"id":5,"title":"I used to think I could fly","nb_tracks":12,"record_type":"album","artist":{"id":42,"name":"Tate McRae"}},
 {"id":6,"title":"So Close To What","nb_tracks":15,"record_type":"album","artist":{"id":42,"name":"Tate McRae"}},
 {"id":7,"title":"Best Of","nb_tracks":30,"record_type":"compile","artist":{"id":42,"name":"Tate McRae"}}
]}`

func TestLookup(t *testing.T) {
    f := &fake{t: t, handler: map[string]string{
        "/search/artist": artistSearch,
        "/artist/42/top": topTracks,
        "/search/album":  albumSearch,
    }}
    c, done := newClient(t, f)
    defer done()

    info, err := c.Lookup(context.Background(), "Tate McRae")
    if err != nil {
        t.Fatal(err)
    }
    if info.Artist != "tate mcrae" {
        t.Errorf("artist %q (debe elegir la coincidencia exacta con más seguidores)", info.Artist)
    }
    // Canciones: sin duplicados por versión, usa title_short si falta title.
    wantTracks := []string{"greedy", "exes", "2 hands"}
    if len(info.Tracks) != len(wantTracks) {
        t.Fatalf("tracks: %+v", info.Tracks)
    }
    for i, w := range wantTracks {
        if info.Tracks[i].Title != w {
            t.Errorf("track %d = %q, want %q", i, info.Tracks[i].Title, w)
        }
    }
    // Álbumes: solo del artista, sin deluxe duplicado, sin singles/compilaciones
    // mientras haya álbumes, máximo 3, en orden de ranking.
    if len(info.Albums) != 3 {
        t.Fatalf("albums: %+v", info.Albums)
    }
    wantAlbums := []string{"THINK LATER (Deluxe)", "I used to think I could fly", "So Close To What"}
    for i, w := range wantAlbums {
        if info.Albums[i].Title != w {
            t.Errorf("album %d = %q, want %q", i, info.Albums[i].Title, w)
        }
    }
    // Parámetros enviados a Deezer
    var sawRanking, sawTop10 bool
    for _, q := range f.queries {
        if q.Get("order") == "RANKING" && strings.Contains(q.Get("q"), `artist:"tate mcrae"`) {
            sawRanking = true
        }
        if q.Get("limit") == "20" {
            sawTop10 = true
        }
    }
    if !sawRanking || !sawTop10 {
        t.Errorf("queries inesperadas: %v", f.queries)
    }
}

func TestNotFoundOnlyPartialMatches(t *testing.T) {
    f := &fake{t: t, handler: map[string]string{
        "/search/artist": `{"data":[{"id":1,"name":"Tate McRae Tribute Band","nb_fan":5}]}`,
    }}
    c, done := newClient(t, f)
    defer done()
    if _, err := c.Lookup(context.Background(), "Tate McRae"); !errors.Is(err, ErrNotFound) {
        t.Fatalf("want ErrNotFound, got %v", err)
    }
}

func TestFeatSplit(t *testing.T) {
    f := &fake{t: t, handler: map[string]string{
        "/search/artist": `{"data":[{"id":7,"name":"Artist A","nb_fan":1}]}`,
        "/artist/7/top":  `{"data":[{"title":"Song","album":{"title":"Alb"}}]}`,
        "/search/album":  `{"data":[]}`,
    }}
    c, done := newClient(t, f)
    defer done()
    info, err := c.Lookup(context.Background(), "Artist A, Artist B")
    if err != nil || info.Artist != "Artist A" || len(info.Tracks) != 1 {
        t.Fatalf("got %+v, %v", info, err)
    }
}

func TestErrors(t *testing.T) {
    // Error de la API con HTTP 200
    f := &fake{t: t, handler: map[string]string{
        "/search/artist": `{"error":{"type":"Exception","message":"Quota limit exceeded","code":4}}`,
    }}
    c, done := newClient(t, f)
    if _, err := c.Lookup(context.Background(), "X"); err == nil || !strings.Contains(err.Error(), "Quota") {
        t.Errorf("api error not reported: %v", err)
    }
    done()

    // HTTP 500
    f = &fake{t: t, status: map[string]int{"/search/artist": 500}}
    c, done = newClient(t, f)
    if _, err := c.Lookup(context.Background(), "X"); err == nil {
        t.Error("500 should fail")
    }
    done()

    // JSON inválido
    f = &fake{t: t, handler: map[string]string{"/search/artist": "<html>"}}
    c, done = newClient(t, f)
    if _, err := c.Lookup(context.Background(), "X"); err == nil {
        t.Error("bad json should fail")
    }
    done()

    // Sin conexión: servidor cerrado
    f = &fake{t: t}
    c, done = newClient(t, f)
    done()
    if _, err := c.Lookup(context.Background(), "X"); err == nil {
        t.Error("offline should fail")
    }

    // Nombre vacío
    if _, err := c.Lookup(context.Background(), "  "); !errors.Is(err, ErrNotFound) {
        t.Errorf("empty name: %v", err)
    }
}

func TestAlbumsFailKeepsTracks(t *testing.T) {
    f := &fake{t: t,
        handler: map[string]string{
            "/search/artist": `{"data":[{"id":7,"name":"A","nb_fan":1}]}`,
            "/artist/7/top":  `{"data":[{"title":"Song","album":{"title":"Alb"}}]}`,
        },
        status: map[string]int{"/search/album": 500},
    }
    c, done := newClient(t, f)
    defer done()
    info, err := c.Lookup(context.Background(), "A")
    if err != nil || len(info.Tracks) != 1 || len(info.Albums) != 0 {
        t.Fatalf("got %+v, %v", info, err)
    }
}

func TestNormalizeTitle(t *testing.T) {
    for in, want := range map[string]string{
        "THINK LATER (Deluxe)":         "think later",
        "Song - Remastered 2011":       "song",
        "Name [Live] (Bonus)":          "name",
        "  Plain  ":                    "plain",
        "(Only Brackets)":              "",
    } {
        if got := normalizeTitle(in); got != want {
            t.Errorf("normalizeTitle(%q)=%q want %q", in, got, want)
        }
    }
}
