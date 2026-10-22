package localmedia

import (
    "context"
    "errors"
    "fmt"
    "log"
    "net"
    "net/http"
    "os"
    "path"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/dhowden/tag"
)

// Item es una pista ya publicada: incluye las URLs que debe usar el dispositivo.
type Item struct {
    Title  string
    Artist string
    Album  string
    Mime   string
    URL    string
    ArtURL string
}

// Server sirve la biblioteca publicada por HTTP. Solo atiende archivos que
// forman parte de la biblioteca actual (nunca rutas arbitrarias) y solo a
// clientes de la red local.
type Server struct {
    preferredPort int

    mu       sync.RWMutex
    tracks   []*Track
    listener net.Listener
    srv      *http.Server
    listenIP string
    port     int
}

func NewServer(preferredPort int) *Server {
    return &Server{preferredPort: preferredPort}
}

// LocalIPFor devuelve la IP local con la que este equipo alcanza a host.
// No envía ningún paquete: solo consulta la tabla de rutas.
func LocalIPFor(host string) (net.IP, error) {
    conn, err := net.Dial("udp", net.JoinHostPort(host, "9"))
    if err != nil {
        return nil, err
    }
    defer conn.Close()
    addr, ok := conn.LocalAddr().(*net.UDPAddr)
    if !ok {
        return nil, errors.New("could not determine local address")
    }
    return addr.IP, nil
}

// Publish reemplaza la biblioteca servida por tracks y devuelve las URLs que
// debe usar el dispositivo en deviceHost (arrancando el servidor si hace falta).
func (s *Server) Publish(tracks []*Track, deviceHost string) ([]Item, error) {
    ip, err := LocalIPFor(deviceHost)
    if err != nil {
        return nil, fmt.Errorf("cannot reach %s: %v", deviceHost, err)
    }

    s.mu.Lock()
    defer s.mu.Unlock()
    if err := s.ensureListening(ip.String()); err != nil {
        return nil, err
    }
    for i, t := range tracks {
        t.ID = i
    }
    s.tracks = tracks

    base := "http://" + net.JoinHostPort(s.listenIP, strconv.Itoa(s.port))
    items := make([]Item, 0, len(tracks))
    for _, t := range tracks {
        ext := strings.ToLower(filepath.Ext(t.Path))
        item := Item{
            Title:  t.Title,
            Artist: t.Artist,
            Album:  t.Album,
            Mime:   t.Mime,
            URL:    fmt.Sprintf("%s/t/%d%s", base, t.ID, ext),
        }
        if t.HasArt() {
            item.ArtURL = fmt.Sprintf("%s/a/%d", base, t.ID)
        }
        items = append(items, item)
    }
    return items, nil
}

// Con s.mu tomado.
func (s *Server) ensureListening(ip string) error {
    if s.listener != nil && s.listenIP == ip {
        return nil
    }
    s.closeLocked()

    ln, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(s.preferredPort)))
    if err != nil && s.preferredPort != 0 {
        log.Printf("Port %d unavailable (%v), using a random port", s.preferredPort, err)
        ln, err = net.Listen("tcp", net.JoinHostPort(ip, "0"))
    }
    if err != nil {
        return err
    }
    mux := http.NewServeMux()
    mux.HandleFunc("/t/", s.handleTrack)
    mux.HandleFunc("/a/", s.handleArt)
    s.srv = &http.Server{Handler: logRequests(mux), ReadHeaderTimeout: 10 * time.Second}
    s.listener = ln
    s.listenIP = ip
    s.port = ln.Addr().(*net.TCPAddr).Port
    log.Printf("Serving local media on %s", ln.Addr())
    go s.srv.Serve(ln)
    return nil
}

func (s *Server) closeLocked() {
    if s.srv != nil {
        ctx, cancel := context.WithTimeout(context.Background(), time.Second)
        s.srv.Shutdown(ctx)
        cancel()
        s.srv, s.listener = nil, nil
    }
}

func (s *Server) Close() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.closeLocked()
}

// allowedClient acepta solo clientes de la red local.
func allowedClient(remoteAddr string) bool {
    host, _, err := net.SplitHostPort(remoteAddr)
    if err != nil {
        return false
    }
    ip := net.ParseIP(host)
    if ip == nil {
        return false
    }
    return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

func (s *Server) lookup(prefix string, r *http.Request) *Track {
    name := strings.TrimPrefix(r.URL.Path, prefix)
    name = strings.TrimSuffix(name, path.Ext(name))
    id, err := strconv.Atoi(name)
    if err != nil {
        return nil
    }
    s.mu.RLock()
    defer s.mu.RUnlock()
    if id < 0 || id >= len(s.tracks) {
        return nil
    }
    return s.tracks[id]
}

func (s *Server) handleTrack(w http.ResponseWriter, r *http.Request) {
    if !allowedClient(r.RemoteAddr) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    t := s.lookup("/t/", r)
    if t == nil {
        http.NotFound(w, r)
        return
    }
    f, err := os.Open(t.Path)
    if err != nil {
        http.NotFound(w, r)
        return
    }
    defer f.Close()
    info, err := f.Stat()
    if err != nil {
        http.NotFound(w, r)
        return
    }
    w.Header().Set("Content-Type", t.Mime)
    w.Header().Set("transferMode.dlna.org", "Streaming")
    // ServeContent gestiona Range (imprescindible para buscar en la pista) y HEAD.
    http.ServeContent(w, r, filepath.Base(t.Path), info.ModTime(), f)
}

func (s *Server) handleArt(w http.ResponseWriter, r *http.Request) {
    if !allowedClient(r.RemoteAddr) {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    t := s.lookup("/a/", r)
    if t == nil || !t.HasArt() {
        http.NotFound(w, r)
        return
    }
    if t.ArtPath != "" {
        http.ServeFile(w, r, t.ArtPath)
        return
    }
    f, err := os.Open(t.Path)
    if err != nil {
        http.NotFound(w, r)
        return
    }
    defer f.Close()
    m, err := tag.ReadFrom(f)
    if err != nil || m == nil || m.Picture() == nil {
        http.NotFound(w, r)
        return
    }
    pic := m.Picture()
    if pic.MIMEType != "" {
        w.Header().Set("Content-Type", pic.MIMEType)
    }
    w.Header().Set("Content-Length", strconv.Itoa(len(pic.Data)))
    w.Write(pic.Data)
}

type statusWriter struct {
    http.ResponseWriter
    status int
    bytes  int64
}

func (w *statusWriter) WriteHeader(code int) {
    w.status = code
    w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
    if w.status == 0 {
        w.status = http.StatusOK
    }
    n, err := w.ResponseWriter.Write(p)
    w.bytes += int64(n)
    return n, err
}

// Registra cada petición del dispositivo: permite saber si el WiiM llegó a
// pedir la pista y qué respondió el servidor.
func logRequests(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sw := &statusWriter{ResponseWriter: w}
        next.ServeHTTP(sw, r)
        log.Printf("localmedia: %s %s from %s range=%q -> %d (%d bytes) UA=%q",
            r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("Range"),
            sw.status, sw.bytes, r.UserAgent())
    })
}
