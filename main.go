package main

import (
	"YouTubeLiveGame/engine/streamer"
	"context"
	_ "embed"
	"fmt"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/labstack/echo/v4"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	width  = 1280
	height = 720
)

//go:embed assets/font.ttf
var fontBytes []byte

func main() {
	font, err := truetype.Parse(fontBytes)
	if err != nil {
		fmt.Println("couldn't parse font file", err)
	}
	//load connect to stream
	fileFrameStreamer, err := streamer.NewFileFrameStreamer("./debug", os.Getenv("STREAM_URL"), width, height)
	if err != nil {
		panic(err)
	}

	fmt.Println("> connected to the streaming service")
	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()

	input := make(chan string, 100)

	//statistics
	frameCounts := 0
	frameDurations := time.Duration(0)
	errorsCount := 0
	titleText := "YouTube Live Based Game Engine!"
	isPaused := false
	avgFrameTime := func() time.Duration {
		if frameCounts == 0 {
			return frameDurations
		}
		return frameDurations / time.Duration(frameCounts)
	}
	lastBangTime := time.Now().Add(-time.Hour)
	// frame generation
	frameNum := 0
	go func() {
		frame := gg.NewContext(width, height)
		// load assets
		largeFont := truetype.NewFace(font, &truetype.Options{Size: 30})
		smallFont := truetype.NewFace(font, &truetype.Options{Size: 12})
		// run
		ticker := time.NewTicker(time.Second / 30)

		// cached

		for {
			<-ticker.C

			frameNum++
			// frame statistics
			if frameNum%100 == 0 {
				errorsCount = 0
				frameDurations = avgFrameTime()
				frameCounts = 1
			}

			// the app logic
			color := float64(frameNum%200) / 1000
			if time.Since(lastBangTime) < time.Second*5 {
				color = float64(time.Since(lastBangTime)) / float64(time.Second) * 5
			}

			startedTime := time.Now()
			frame.SetRGB(color, color, color)
			frame.Clear()

			frame.SetRGB(1, 1, 1)
			frame.SetFontFace(largeFont)
			frame.DrawStringWrapped(titleText, 50, 100, 0, 0, float64(width)-50, 1.5, gg.AlignLeft)

			frame.SetFontFace(smallFont)
			frame.DrawString(fmt.Sprintf("frame: %d", frameNum), 10, 22)
			if frameCounts > 0 {
				frame.DrawString(fmt.Sprintf("avg frame time: %s", avgFrameTime().String()), 10, 42)
			}

			frame.DrawCircle(float64(frameNum*2%width), 600, 50)
			frame.Fill()

			if !isPaused {
				// save and publish
				if err := fileFrameStreamer.Stream(frame.Image()); err != nil {
					fmt.Println("error happened", err)
					errorsCount++
					if errorsCount > 5 {
						fmt.Println("too many errors!")
						stop()
						return
					}
				}
			}
			frameDurations += time.Since(startedTime)
			frameCounts += 1

			// handle inputs

			select {
			case line := <-input:
				args := strings.Fields(line)
				if len(args) == 0 {
					continue
				}
				fmt.Println("\n\nResult for ", args[0])
				if args[0] == "info" {
					fmt.Println("#", frameNum)
					fmt.Println("average frame duration:", frameDurations/time.Duration(frameCounts))
					fmt.Println("average error count:", errorsCount)
				}

				if args[0] == "capture" {
					if err := frame.SavePNG("./debug/capture.png"); err != nil {
						fmt.Println("error happened", err)
					} else {
						fmt.Println("captured successfully")
					}
				}

				if args[0] == "setText" {
					if len(args) == 1 {
						continue
					}
					titleText = strings.Join(args[1:], " ")
				}

				if args[0] == "toggle" {
					isPaused = !isPaused
				}

				if args[0] == "bang" {
					lastBangTime = time.Now()
				}

			default:
			}
		}
	}()

	chatIdChannel := make(chan string)
	go func() {
		chatId := <-chatIdChannel
		fmt.Println("received chat id", chatId)
		service, err := youtube.NewService(context.Background(), option.WithAPIKey(os.Getenv("YOUTUBE_LIVE_API")))
		if err != nil {
			panic(err)
		}

		errCount := 0
		for {
			if errCount > 10 {
				fmt.Println("too many errors!")
				errCount = 0
				<-time.After(time.Minute)
			}
			if err := service.LiveChatMessages.List(chatId, []string{"snippet", "authorDetails"}).Pages(context.Background(), func(response *youtube.LiveChatMessageListResponse) error {
				for _, item := range response.Items {
					message := strings.ToLower(item.Snippet.TextMessageDetails.MessageText)
					if message == "bang" {
						select {
						case input <- "bang":
						default:
						}
					}
				}
				<-time.After(time.Second * 3)
				return nil
			}); err != nil {
				errCount++
				fmt.Println("error happened ", err, " count ", errCount, " waiting for 10 seconds")
				<-time.After(time.Second * 10)
			}
		}
	}()

	go func() {
		e := echo.New()
		e.GET("/info", func(c echo.Context) error {
			return c.JSON(200, map[string]interface{}{
				"ok":                 true,
				"frame":              frameNum,
				"avg_frame_duration": avgFrameTime().String(),
			})
		})

		e.GET("/setText", func(c echo.Context) error {
			txt := c.QueryParam("text")
			input <- "setText " + txt
			return c.JSON(200, map[string]interface{}{
				"ok": true,
			})
		})

		e.GET("/setChatId", func(c echo.Context) error {
			txt := c.QueryParam("id")
			chatIdChannel <- txt
			return c.JSON(200, map[string]interface{}{
				"ok": true,
			})
		})

		e.GET("/bang", func(c echo.Context) error {
			input <- "bang"
			return c.JSON(200, map[string]interface{}{
				"ok": true,
			})
		})

		port := os.Getenv("PORT")
		if port == "" {
			port = ":8081"
		}
		e.Logger.Error(e.Start(port))
	}()

	<-appCtx.Done()
	fmt.Println("> closing application")
	if err := fileFrameStreamer.Close(); err != nil {
		panic(err)
	}
}
