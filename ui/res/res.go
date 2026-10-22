package res

import (
    _ "embed"
)

//go:embed default_album.png
var DefaultAlbum []byte

//go:embed default_track.png
var DefaultTrack []byte
