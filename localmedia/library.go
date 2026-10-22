// Package localmedia escanea una carpeta de música local y la publica por HTTP
// en la red local para que un dispositivo UPnP/DLNA (el WiiM) pueda reproducirla.
package localmedia

import (
    "errors"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "github.com/dhowden/tag"
)

// Track es un archivo de audio de la biblioteca local.
type Track struct {
    ID          int
    Path        string
    Title       string
    Artist      string
    Album       string
    Mime        string
    ArtPath     string // imagen de portada en la carpeta (cover.jpg, folder.png...)
    EmbeddedArt bool   // la portada está incrustada en las etiquetas del archivo
}

func (t *Track) HasArt() bool {
    return t.ArtPath != "" || t.EmbeddedArt
}

var audioTypes = map[string]string{
    ".flac": "audio/flac",
    ".mp3":  "audio/mpeg",
    ".wav":  "audio/wav",
    ".m4a":  "audio/mp4",
    ".aac":  "audio/aac",
    ".ogg":  "audio/ogg",
    ".oga":  "audio/ogg",
    ".opus": "audio/ogg",
    ".aif":  "audio/aiff",
    ".aiff": "audio/aiff",
}

var artNames = []string{
    "cover.jpg", "cover.jpeg", "cover.png",
    "folder.jpg", "folder.jpeg", "folder.png",
    "front.jpg", "front.jpeg", "front.png",
    "albumart.jpg", "albumart.png",
}

// Scan recorre root de forma recursiva y devuelve los archivos de audio
// ordenados por ruta en orden "natural" (2.flac antes que 10.flac).
func Scan(root string) ([]*Track, error) {
    info, err := os.Stat(root)
    if err != nil {
        return nil, err
    }
    if !info.IsDir() {
        return nil, fmt.Errorf("%s is not a folder", root)
    }

    var paths []string
    filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil // carpetas ilegibles se omiten
        }
        name := d.Name()
        if strings.HasPrefix(name, ".") && path != root {
            if d.IsDir() {
                return filepath.SkipDir
            }
            return nil // archivos ocultos (p. ej. ._cancion.flac de macOS)
        }
        if d.IsDir() {
            return nil
        }
        if _, ok := audioTypes[strings.ToLower(filepath.Ext(name))]; ok {
            paths = append(paths, path)
        }
        return nil
    })
    if len(paths) == 0 {
        return nil, errors.New("no audio files found")
    }
    sort.SliceStable(paths, func(i, j int) bool {
        return naturalLess(paths[i], paths[j])
    })

    artCache := map[string]string{}
    tracks := make([]*Track, 0, len(paths))
    for i, path := range paths {
        t := &Track{
            ID:   i,
            Path: path,
            Mime: audioTypes[strings.ToLower(filepath.Ext(path))],
        }
        readTags(t)
        dir := filepath.Dir(path)
        art, ok := artCache[dir]
        if !ok {
            art = findFolderArt(dir)
            artCache[dir] = art
        }
        if !t.EmbeddedArt {
            t.ArtPath = art
        }
        tracks = append(tracks, t)
    }
    return tracks, nil
}

func readTags(t *Track) {
    stem := strings.TrimSuffix(filepath.Base(t.Path), filepath.Ext(t.Path))
    t.Title = stem
    t.Album = filepath.Base(filepath.Dir(t.Path))

    f, err := os.Open(t.Path)
    if err != nil {
        return
    }
    defer f.Close()

    m, err := tag.ReadFrom(f)
    if err != nil || m == nil {
        return // WAV/AIFF y archivos sin etiquetas: se queda con nombre de archivo
    }
    if v := strings.TrimSpace(m.Title()); v != "" {
        t.Title = v
    }
    if v := strings.TrimSpace(m.Artist()); v != "" {
        t.Artist = v
    } else if v := strings.TrimSpace(m.AlbumArtist()); v != "" {
        t.Artist = v
    }
    if v := strings.TrimSpace(m.Album()); v != "" {
        t.Album = v
    }
    if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
        t.EmbeddedArt = true
    }
}

func findFolderArt(dir string) string {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return ""
    }
    byLower := map[string]string{}
    for _, e := range entries {
        if !e.IsDir() {
            byLower[strings.ToLower(e.Name())] = e.Name()
        }
    }
    for _, name := range artNames {
        if real, ok := byLower[name]; ok {
            return filepath.Join(dir, real)
        }
    }
    return ""
}

// naturalLess compara ignorando mayúsculas y tratando los bloques de dígitos
// como números ("track2" < "track10").
func naturalLess(a, b string) bool {
    a, b = strings.ToLower(a), strings.ToLower(b)
    i, j := 0, 0
    for i < len(a) && j < len(b) {
        if isDigit(a[i]) && isDigit(b[j]) {
            si := i
            for i < len(a) && isDigit(a[i]) {
                i++
            }
            sj := j
            for j < len(b) && isDigit(b[j]) {
                j++
            }
            na := strings.TrimLeft(a[si:i], "0")
            nb := strings.TrimLeft(b[sj:j], "0")
            if len(na) != len(nb) {
                return len(na) < len(nb)
            }
            if na != nb {
                return na < nb
            }
            continue
        }
        if a[i] != b[j] {
            return a[i] < b[j]
        }
        i++
        j++
    }
    return len(a)-i < len(b)-j
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
