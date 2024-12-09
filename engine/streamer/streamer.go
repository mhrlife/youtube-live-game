package streamer

import "image"

// FrameStreamer is an interface for streaming frames to different outputs.
type FrameStreamer interface {
	Stream(frame image.Image) error
	Close() error
}
