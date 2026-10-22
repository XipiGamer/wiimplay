// Package artistinfo consulta la API pública de Deezer (sin clave) para obtener
// los álbumes y canciones más populares de un artista. "Popular" es el ranking
// de reproducciones de Deezer; no son cifras de ventas.
package artistinfo

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "net/url"
    "regexp"
    "strconv"
    "strings"
    "time"
)

const (
    defaultBase = "https://api.deezer.com"
    MaxAlbums   = 3
    MaxTracks   = 10
)

var ErrNotFound = errors.New("artist not found")

type Album struct {
    Title  string
    Tracks int
}

type Track struct {
    Title string
    Album string
}

type Info struct {
    Artist string // nombre tal como lo conoce Deezer
    Albums []Album
    Tracks []Track
}

type Client struct {
    base string
    http *http.Client
}

func NewClient() *Client {
    return &Client{
        base: defaultBase,
        http: &http.Client{Timeout: 8 * time.Second},
    }
}

// ---- Formas JSON de Deezer (todos los campos son opcionales) ----

type apiError struct {
    Type    string `json:"type"`
    Message string `json:"message"`
    Code    int    `json:"code"`
}

type apiArtist struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    NbFan int    `json:"nb_fan"`
}

type apiAlbum struct {
    ID         int64     `json:"id"`
    Title      string    `json:"title"`
    NbTracks   int       `json:"nb_tracks"`
    RecordType string    `json:"record_type"`
    Artist     apiArtist `json:"artist"`
}

type apiTrack struct {
    Title      string `json:"title"`
    TitleShort string `json:"title_short"`
    Album      struct {
        Title string `json:"title"`
    } `json:"album"`
}

type apiList[T any] struct {
    Data  []T       `json:"data"`
    Error *apiError `json:"error"`
}

func get[T any](ctx context.Context, c *Client, path string, query url.Values, out *apiList[T]) error {
    u := c.base + path
    if len(query) > 0 {
        u += "?" + query.Encode()
    }
    req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
    if err != nil {
        return err
    }
    resp, err := c.http.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("deezer: HTTP %d", resp.StatusCode)
    }
    if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
        return fmt.Errorf("deezer: bad response: %v", err)
    }
    // Deezer devuelve los errores de la API con HTTP 200.
    if out.Error != nil {
        return fmt.Errorf("deezer: %s (code %d)", out.Error.Message, out.Error.Code)
    }
    return nil
}

// Lookup busca el artista por nombre y devuelve sus álbumes y canciones más
// populares. Devuelve ErrNotFound si no hay una coincidencia fiable.
func (c *Client) Lookup(ctx context.Context, name string) (*Info, error) {
    artist, err := c.findArtist(ctx, name)
    if err != nil {
        return nil, err
    }
    info := &Info{Artist: artist.Name}

    // Canciones populares del artista.
    var tops apiList[apiTrack]
    q := url.Values{"limit": {strconv.Itoa(MaxTracks * 2)}}
    if err := get(ctx, c, "/artist/"+strconv.FormatInt(artist.ID, 10)+"/top", q, &tops); err != nil {
        return nil, err
    }
    seen := map[string]bool{}
    for _, t := range tops.Data {
        title := t.Title
        if title == "" {
            title = t.TitleShort
        }
        key := normalizeTitle(title)
        if key == "" || seen[key] {
            continue
        }
        seen[key] = true
        info.Tracks = append(info.Tracks, Track{Title: title, Album: t.Album.Title})
        if len(info.Tracks) == MaxTracks {
            break
        }
    }

    // Álbumes ordenados por popularidad (ranking de Deezer).
    var albums apiList[apiAlbum]
    q = url.Values{
        "q":     {fmt.Sprintf(`artist:"%s"`, strings.ReplaceAll(artist.Name, `"`, ""))},
        "order": {"RANKING"},
        "limit": {"50"},
    }
    if err := get(ctx, c, "/search/album", q, &albums); err != nil {
        // Las canciones ya están: se devuelve lo que haya.
        if len(info.Tracks) > 0 {
            return info, nil
        }
        return nil, err
    }
    info.Albums = pickAlbums(albums.Data, artist.ID)

    if len(info.Albums) == 0 && len(info.Tracks) == 0 {
        return nil, ErrNotFound
    }
    return info, nil
}

func (c *Client) findArtist(ctx context.Context, name string) (*apiArtist, error) {
    for _, candidate := range nameCandidates(name) {
        var res apiList[apiArtist]
        q := url.Values{"q": {candidate}, "limit": {"10"}}
        if err := get(ctx, c, "/search/artist", q, &res); err != nil {
            return nil, err
        }
        // Solo coincidencia exacta (sin distinguir mayúsculas ni acentos básicos);
        // entre varias, la de más seguidores. Una coincidencia parcial podría
        // mostrar datos de otro artista.
        var best *apiArtist
        for i := range res.Data {
            a := &res.Data[i]
            if strings.EqualFold(strings.TrimSpace(a.Name), candidate) {
                if best == nil || a.NbFan > best.NbFan {
                    best = a
                }
            }
        }
        if best != nil {
            return best, nil
        }
    }
    return nil, ErrNotFound
}

var splitters = regexp.MustCompile(`(?i)\s*(?:,|;|\sfeat\.?\s|\sft\.?\s|\sfeaturing\s)\s*`)

// Nombres a probar: el completo y, si es una lista ("A, B", "A feat. B"), el primero.
func nameCandidates(name string) []string {
    name = strings.TrimSpace(name)
    if name == "" {
        return nil
    }
    out := []string{name}
    parts := splitters.Split(name, -1)
    if len(parts) > 1 && strings.TrimSpace(parts[0]) != "" {
        out = append(out, strings.TrimSpace(parts[0]))
    }
    return out
}

var noise = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*[\)\]]|\s+-\s+.*$`)

// Normaliza un título para detectar duplicados: "X (Deluxe)" y "X - Remastered" -> "x".
func normalizeTitle(title string) string {
    t := noise.ReplaceAllString(title, "")
    return strings.ToLower(strings.TrimSpace(t))
}

// Elige los álbumes más populares: solo del artista, preferentemente de tipo
// "album", sin duplicados por edición (deluxe, remasterizado...).
func pickAlbums(list []apiAlbum, artistID int64) []Album {
    var own []apiAlbum
    for _, a := range list {
        if a.Artist.ID != 0 && a.Artist.ID != artistID {
            continue
        }
        own = append(own, a)
    }
    seen := map[string]bool{}
    var out []Album
    take := func(match func(apiAlbum) bool) {
        for _, a := range own {
            if len(out) == MaxAlbums {
                return
            }
            key := normalizeTitle(a.Title)
            if key == "" || seen[key] || !match(a) {
                continue
            }
            seen[key] = true
            out = append(out, Album{Title: a.Title, Tracks: a.NbTracks})
        }
    }
    take(func(a apiAlbum) bool { return strings.EqualFold(a.RecordType, "album") })
    take(func(a apiAlbum) bool { return !strings.EqualFold(a.RecordType, "compile") })
    return out
}
