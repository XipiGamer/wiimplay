package wiimdev

import (
    "time"
)

type Device interface {
    GetInfo() (*DeviceInfo, error)
    Play() error
    Pause() error
    Previous() error
    Next() error
    LoopMode(loopMode LoopMode) error
    Volume(level int) error
    Seek(position time.Duration) error
    Mute(mute bool) error
    GetPlayLists() ([]PlayList, string, error)
    GetTracks(id string) ([]Track, int, error)
    PlayTrack(track int) error
    SetPlayList(id string) error
    PlayMedia(name string, items []MediaItem) (bool, error)
    PlayQueueTrack(id string, index int) error
    GetDeviceDiscovery() *DeviceDiscovery
    SetEqualizer(setting string) error
    GetEqualizer() (string, error)
    GetEqualizerList() ([]string, error)
    GetAudioOutput() (int, error)
    GetAudioOutputList() ([]string, error)
    SetAudioOutput(output int) error
    GetBalance() (float64, error)
    SetBalance(level float64) error
    GetFadeEffects() (bool, error)
    SetFadeEffects(on bool) error
    GetFixedVolume() (bool, error)
    SetFixedVolume(on bool) error
    GetAudioInput() (int, error)
    GetAudioInputList() ([]string, error)
    SetAudioInput(input int) error
}

type DeviceDiscovery struct {
    Url string
    Name string
    ModelNumber string
    ModelName string
    Uuid string
}

type LoopMode uint
const (
    LoopModeNone LoopMode = 4
    LoopModeOne = 1
    LoopModeAll = 0
    LoopModeNoneShuffle = 3
    LoopModeOneShuffle = 5
    LoopModeAllShuffle = 2
)

type TransportState uint
const (
    StateUnknown TransportState = iota
    StateStopped
    StatePaused
    StatePlaying
    StateTransitioning
    StateNoMedia
)

// Origen de la reproducción según el WiiM (GetInfoEx). Los valores son los que
// informa el dispositivo, sin interpretar.
type SourceInfo struct {
    PlayMedium    string
    TrackSource   string
    PlayType      string
    TrackURI      string
    SpotifyActive bool
}

type DeviceInfo struct {
    TransportState TransportState
    Track int
    TrackTotal time.Duration
    TrackUsed time.Duration
    Volume int
    Mute bool
    LoopMode LoopMode
    TrackMetadata *Metadata
    Source SourceInfo
}

type Metadata struct {
    Id       string
    Title    string
    Album    string
    Artist   string
    AlbumArt string
    Rate     int
    Format   string
    Quality  string
    BitRate  int
    Error    error
}

type PlayList struct {
    Id string
    Name string
    Source string
}

type Track struct {
    Id string
    Source string
    Metadata *Metadata
}
