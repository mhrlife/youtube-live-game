package streamer

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrAbort = errors.New("abort")
)

// FileFrameStreamer implements the FrameStreamer interface and streams frames to a local file using FFmpeg.
type FileFrameStreamer struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	outputDir     string // Directory for output files
	width, height int

	streamURL string // URL for streaming

	isReconnecting   bool // Flag to indicate if the streamer is reconnecting
	isAbort          bool
	reconnectLock    sync.Mutex    // Mutex to handle concurrent access to reconnection logic
	errorBucket      time.Time     // Number of consecutive reconnection attempts
	maxReconnects    int           // Maximum number of consecutive reconnect attempts allowed
	reconnectTimeout time.Duration // Time duration to wait before reconnecting
}

// NewFileFrameStreamer initializes FFmpeg and prepares it to receive frames for local file debug.
func NewFileFrameStreamer(
	outputDir, streamURL string,
	width, height int,
) (*FileFrameStreamer, error) {
	// Ensure the debug directory exists
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return nil, err
	}

	// Create a new FileFrameStreamer with the initial parameters
	streamer := &FileFrameStreamer{
		outputDir:        outputDir,
		width:            width,
		height:           height,
		maxReconnects:    2,               // Allow up to 10 consecutive reconnection attempts
		streamURL:        streamURL,       // Set the streaming URL
		reconnectTimeout: 2 * time.Second, // 5 seconds between reconnect attempts
		errorBucket:      time.Now(),
	}

	// Initialize FFmpeg and start streaming
	if err := streamer.startFFmpeg(); err != nil {
		return nil, err
	}

	return streamer, nil
}

// startFFmpeg starts the FFmpeg command for streaming.
func (s *FileFrameStreamer) startFFmpeg() error {
	// Configure the FFmpeg command to write to a local file
	cmd := exec.Command("bash", "-c", `
ffmpeg -y \
    -f rawvideo \
    -pixel_format rgba \
    -video_size `+fmt.Sprintf("%dx%d", s.width, s.height)+` \
    -r 30 \
    -i - \
    -f lavfi -i anullsrc=r=44100:cl=stereo \
    -g 60 \
    -pix_fmt yuv420p \
    -vcodec libx264 \
    -preset ultrafast \
    -crf 23 \
    -threads 2 \
    -b:v 3000k \
    -maxrate 3000k \
    -bufsize 1500k \
    -b:a 128k \
	-c:a mp3 \
	-async 1 \
	-vsync vfr \
	-ac 2 \
	-ar 44100 \
	-x264-params "scenecut=0:open_gop=0:min-keyint=60:keyint=60" \
	-x264opts "cabac=1:ref=1:bframes=2" \
    -tune zerolatency \
    -f flv \
	-reconnect 1 -reconnect_at_eof 1 -reconnect_streamed 1 -reconnect_delay_max 10 \
    "`+s.streamURL+`" \
    > ./debug/std.txt 2> ./debug/err.`+fmt.Sprintf("%d", time.Now().Unix())+`.txt
`)

	// Create pipes for FFmpeg input and outputs
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	// Start the FFmpeg process
	if err := cmd.Start(); err != nil {
		return err
	}

	// Assign the new command and stdin to the FileFrameStreamer
	s.cmd = cmd
	s.stdin = stdin
	s.isReconnecting = false // Reset reconnecting flag

	return nil
}

// reconnect tries to gracefully restart the FFmpeg process.
func (s *FileFrameStreamer) reconnect() error {
	fmt.Println("attempting to reconnect #", s.timeoutBucket().Sub(time.Now()))
	s.reconnectLock.Lock()
	defer s.reconnectLock.Unlock()

	if s.isReconnecting {
		return fmt.Errorf("already reconnecting")
	}
	s.isReconnecting = true

	// Increment the reconnect counter and check if it exceeds the maximum allowed
	s.errorBucket = s.timeoutBucket().Add(time.Minute)
	if s.timeoutBucket().After(time.Now().Add(time.Minute * 5)) {
		return ErrAbort
	}

	fmt.Println("closing the ffmpeg process")
	// Attempt to close the current FFmpeg process
	s.Close()

	fmt.Println("waiting for ", s.reconnectTimeout, " timeout")
	// Wait for a short period before reconnecting
	time.Sleep(s.reconnectTimeout)

	fmt.Println("trying to reconnect to ffmpeg")
	// Try to restart FFmpeg
	if err := s.startFFmpeg(); err != nil {
		s.isReconnecting = false
		return fmt.Errorf("failed to reconnect: %v", err)
	}

	fmt.Println("the new ffmpeg process is running. hoping for a new stream to be successful.")
	// Reset reconnection state and counter on successful reconnect
	s.isReconnecting = false
	return nil
}

// Stream sends a single frame to FFmpeg via stdin in raw frame format.
func (s *FileFrameStreamer) Stream(frame image.Image) error {
	if s.isAbort {
		return ErrAbort
	}
	// If reconnecting, ignore the input frame
	if s.isReconnecting {
		return nil
	}

	// Cast the image to RGBA
	rgbaFrame, ok := frame.(*image.RGBA)
	if !ok {
		return fmt.Errorf("frame is not an RGBA image")
	}

	// Write the raw RGBA data directly to FFmpeg's stdin
	if _, err := s.stdin.Write(rgbaFrame.Pix); err != nil {
		// Check if the error is a broken pipe error and attempt to reconnect
		if strings.Contains(err.Error(), "broken pipe") {
			fmt.Println("Broken pipe detected, attempting to reconnect...")
			go func() {
				if reconnectErr := s.reconnect(); reconnectErr != nil {
					if errors.Is(reconnectErr, ErrAbort) {
						s.isAbort = true
					}
					fmt.Printf("failed to reconnect: %v\n", reconnectErr)
					return
				}
				fmt.Println("reconnected successfully!")
			}()
			return nil
		}
		return fmt.Errorf("failed to write raw frame data: %v", err)
	}
	return nil
}

// Close gracefully closes the FFmpeg process and pipes.
func (s *FileFrameStreamer) Close() error {
	// Lock during close operation to prevent concurrent writes
	fmt.Println("closing the file frame streamer process")

	if s.stdin != nil {
		fmt.Println("closing the stdin")
		if err := s.stdin.Close(); err != nil {
			return err
		}
	}
	if s.cmd != nil {
		fmt.Println("closing the cmd")
		if err := s.cmd.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileFrameStreamer) timeoutBucket() time.Time {
	if time.Now().Before(s.errorBucket) {
		return s.errorBucket
	}
	return time.Now()
}
