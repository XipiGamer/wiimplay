package localmedia

import (
    "io"
    "net/http"
    "os"
    "path/filepath"
    "testing"
)

func write(t *testing.T, path string, data string) {
    t.Helper()
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(path, []byte(data), 0644); err != nil {
        t.Fatal(err)
    }
}

func TestNaturalLess(t *testing.T) {
    cases := []struct{ a, b string; want bool }{
        {"2.flac", "10.flac", true},
        {"10.flac", "2.flac", false},
        {"a/02 x.flac", "a/2 y.flac", true},
        {"a/02 y.flac", "a/2 x.flac", false},
        {"Track 9", "track 10", true},
        {"abc", "abd", true},
        {"abc", "abc", false},
        {"a", "ab", true},
    }
    for _, c := range cases {
        if got := naturalLess(c.a, c.b); got != c.want {
            t.Errorf("naturalLess(%q,%q)=%v want %v", c.a, c.b, got, c.want)
        }
    }
}

func TestScan(t *testing.T) {
    root := t.TempDir()
    write(t, filepath.Join(root, "Album A", "10 ten.flac"), "x")
    write(t, filepath.Join(root, "Album A", "2 two.FLAC"), "x")
    write(t, filepath.Join(root, "Album A", "cover.JPG"), "img")
    write(t, filepath.Join(root, "Album A", "notes.txt"), "x")
    write(t, filepath.Join(root, "Album A", "._hidden.flac"), "x")
    write(t, filepath.Join(root, ".trash", "no.flac"), "x")
    write(t, filepath.Join(root, "Album B", "song.mp3"), "x")

    tracks, err := Scan(root)
    if err != nil {
        t.Fatal(err)
    }
    if len(tracks) != 3 {
        t.Fatalf("got %d tracks, want 3", len(tracks))
    }
    want := []string{"2 two.FLAC", "10 ten.flac", "song.mp3"}
    for i, w := range want {
        if filepath.Base(tracks[i].Path) != w {
            t.Errorf("track %d = %s, want %s", i, filepath.Base(tracks[i].Path), w)
        }
    }
    if tracks[0].Title != "2 two" || tracks[0].Album != "Album A" {
        t.Errorf("fallback tags wrong: %q / %q", tracks[0].Title, tracks[0].Album)
    }
    if tracks[0].Mime != "audio/flac" || tracks[2].Mime != "audio/mpeg" {
        t.Errorf("mime wrong: %s %s", tracks[0].Mime, tracks[2].Mime)
    }
    if filepath.Base(tracks[0].ArtPath) != "cover.JPG" || tracks[2].ArtPath != "" {
        t.Errorf("folder art wrong: %q %q", tracks[0].ArtPath, tracks[2].ArtPath)
    }
}

func TestScanErrors(t *testing.T) {
    if _, err := Scan(t.TempDir()); err == nil {
        t.Error("empty folder should fail")
    }
    if _, err := Scan("/nonexistent/folder"); err == nil {
        t.Error("missing folder should fail")
    }
}

func TestServer(t *testing.T) {
    root := t.TempDir()
    content := "0123456789abcdefghij"
    write(t, filepath.Join(root, "a.flac"), content)
    write(t, filepath.Join(root, "cover.png"), "PNGDATA")

    tracks, err := Scan(root)
    if err != nil {
        t.Fatal(err)
    }
    srv := NewServer(0)
    defer srv.Close()
    items, err := srv.Publish(tracks, "127.0.0.1")
    if err != nil {
        t.Fatal(err)
    }
    if len(items) != 1 || items[0].ArtURL == "" {
        t.Fatalf("bad items: %+v", items)
    }

    // Archivo completo
    resp, err := http.Get(items[0].URL)
    if err != nil {
        t.Fatal(err)
    }
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()
    if resp.StatusCode != 200 || string(body) != content {
        t.Fatalf("full get: %d %q", resp.StatusCode, body)
    }
    if ct := resp.Header.Get("Content-Type"); ct != "audio/flac" {
        t.Errorf("content-type %q", ct)
    }

    // Range (necesario para buscar dentro de la pista)
    req, _ := http.NewRequest("GET", items[0].URL, nil)
    req.Header.Set("Range", "bytes=5-9")
    resp, err = http.DefaultClient.Do(req)
    if err != nil {
        t.Fatal(err)
    }
    body, _ = io.ReadAll(resp.Body)
    resp.Body.Close()
    if resp.StatusCode != 206 || string(body) != "56789" {
        t.Fatalf("range get: %d %q", resp.StatusCode, body)
    }

    // HEAD
    resp, err = http.Head(items[0].URL)
    if err != nil || resp.StatusCode != 200 {
        t.Fatalf("head: %v %v", err, resp)
    }
    resp.Body.Close()

    // Portada
    resp, err = http.Get(items[0].ArtURL)
    if err != nil {
        t.Fatal(err)
    }
    body, _ = io.ReadAll(resp.Body)
    resp.Body.Close()
    if resp.StatusCode != 200 || string(body) != "PNGDATA" {
        t.Fatalf("art: %d %q", resp.StatusCode, body)
    }

    // Rutas fuera de la biblioteca o inválidas -> 404
    base := items[0].URL[:len(items[0].URL)-len("/t/0.flac")]
    for _, p := range []string{"/t/99.flac", "/t/abc", "/t/../../etc/passwd", "/a/99", "/t/-1.flac"} {
        resp, err := http.Get(base + p)
        if err != nil {
            t.Fatal(err)
        }
        resp.Body.Close()
        if resp.StatusCode != 404 {
            t.Errorf("%s -> %d, want 404", p, resp.StatusCode)
        }
    }

    // Republicar reemplaza la biblioteca
    write(t, filepath.Join(root, "b.flac"), "second")
    tracks, _ = Scan(root)
    items, err = srv.Publish(tracks, "127.0.0.1")
    if err != nil || len(items) != 2 {
        t.Fatalf("republish: %v %d", err, len(items))
    }
}

func TestAllowedClient(t *testing.T) {
    ok := []string{"192.168.1.50:1234", "10.0.0.2:80", "172.16.5.5:9", "127.0.0.1:1", "[fe80::1]:5", "[::1]:5"}
    bad := []string{"8.8.8.8:80", "1.2.3.4:5", "[2001:4860:4860::8888]:80", "garbage", ""}
    for _, a := range ok {
        if !allowedClient(a) {
            t.Errorf("%s should be allowed", a)
        }
    }
    for _, a := range bad {
        if allowedClient(a) {
            t.Errorf("%s should be rejected", a)
        }
    }
}
