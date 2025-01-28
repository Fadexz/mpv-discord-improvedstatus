package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tnychn/mpv-discord/discordrpc"
	"github.com/tnychn/mpv-discord/mpvrpc"
)

var (
	client   *mpvrpc.Client
	presence *discordrpc.Presence
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Lmsgprefix)

	client = mpvrpc.NewClient()
	presence = discordrpc.NewPresence(os.Args[2])
}

var currTime int64 = time.Now().Local().UnixMilli()

func refreshCurrTime() {
	currTime = time.Now().Local().UnixMilli()
}

func getActivity() (activity discordrpc.Activity, err error) {
	getProperty := func(key string) (prop interface{}) {
		prop, err = client.GetProperty(key)
		return
	}
	getPropertyString := func(key string) (prop string) {
		prop, err = client.GetPropertyString(key)
		return
	}

	// Large Image
	activity.LargeImageKey = "mpv"
	if version := getPropertyString("mpv-version"); version != "" {
		activity.LargeImageText = strings.SplitN(version, "-", 2)[0]
	} else {
		activity.LargeImageText = "mpv"
	}

	// Details
	mediaTitle := getPropertyString("media-title")
	metaTitle := getProperty("metadata/by-key/Title")
	metaArtist := getProperty("metadata/by-key/Artist")
	metaAlbum := getProperty("metadata/by-key/Album")
	speed := getPropertyString("speed")
	height := getPropertyString("height")
	containerFps := getPropertyString("container-fps")
	videoCodec := getPropertyString("video-format")
	videoBitrate := getPropertyString("video-bitrate")
	audioCodec := getPropertyString("audio-codec-name")
	audioBitrate := getPropertyString("audio-bitrate")
	streamPath := getPropertyString("stream-path")
	fileSize := getPropertyString("file-size")
	if metaTitle != nil {
		activity.Details = metaTitle.(string)
	} else {
		activity.Details = mediaTitle
	}

	// State
	if metaArtist != nil {
		activity.Details += " by " + metaArtist.(string)
	}
	if metaAlbum != nil {
		activity.Details += " on " + metaAlbum.(string)
	}
	if speedValue, err := strconv.ParseFloat(speed, 64); err == nil {
		if speedValue != 1 {
			activity.State += fmt.Sprintf(" (x%s)", strconv.FormatFloat(speedValue, 'f', -1, 64))
		}
	}
	if height != "" {
		activity.State += " " + height + "p"
	}
	trimmedFps := ""
	if containerFps != "" {
		if fpsValue, err := strconv.ParseFloat(containerFps, 64); err == nil {
			formattedFps := strconv.FormatFloat(fpsValue, 'f', 3, 64)
			trimmedFps = strings.TrimRight(strings.TrimRight(formattedFps, "0"), ".")
			activity.State += " " + trimmedFps + "fps"
		}
	}
	if videoCodec != "" {
		switch videoCodec {
		case "prores":
			videoCodec = "ProRes"
		case "dnxhd":
			videoCodec = "DNxHD"
		case "dnxhr":
			videoCodec = "DNxHR"
		case "cfhd":
			videoCodec = "CineForm"
		case "mpeg2video":
			videoCodec = "MPEG-2"
		default:
			videoCodec = strings.ToUpper(videoCodec)
		}
		activity.State += " " + videoCodec
	}
	if bitrateValue, err := strconv.Atoi(videoBitrate); err == nil {
		activity.State += fmt.Sprintf(" (%d mbps)", bitrateValue/1000/1000)
	}
	//log.Println("Error:", err)
	if audioCodec != "" {
		switch {
		case audioCodec == "opus" || audioCodec == "vorbis":
			if len(audioCodec) > 0 {
				audioCodec = strings.ToUpper(string(audioCodec[0])) + audioCodec[1:]
			}
		case strings.HasPrefix(audioCodec, "pcm_"):
			audioCodec = "PCM"
		case audioCodec == "truehd":
			audioCodec = "TrueHD"
		default:
			audioCodec = strings.ToUpper(audioCodec)
		}
		activity.State += " " + audioCodec
	}
	if bitrateValue, err := strconv.Atoi(audioBitrate); err == nil {
		activity.State += fmt.Sprintf(" (%d kbps)", bitrateValue/1000)
	}
	if fileSize != "" {
		if fileSizeBytes, err := strconv.ParseFloat(fileSize, 64); err == nil {
			if streamPath == "" || (streamPath != "" && mediaTitle != "index.m3u8" && fileSizeBytes >= 120000) {
				fileSizeBytes = fileSizeBytes / 1024
				sizeMiB := fileSizeBytes / 1024
				sizeGiB := sizeMiB / 1024
				if fileSizeBytes < 1000 {
					activity.State += fmt.Sprintf(" %.1f MiB", sizeMiB)
				} else if sizeMiB < 1000 {
					activity.State += fmt.Sprintf(" %d MiB", int(math.Floor(sizeMiB+0.5)))
				} else if sizeGiB < 10 {
					activity.State += fmt.Sprintf(" %.1f GiB", sizeGiB)
				} else {
					activity.State += fmt.Sprintf(" %d GiB", int(math.Floor(sizeGiB+0.5)))
				}
			}
		}
	}

	// Set Watching/Listening state
	if videoCodec == "" {
		activity.Type = 2
	} else {
		activity.Type = 3
	}

	// Small Image
	buffering := getProperty("paused-for-cache")
	pausing := getProperty("pause")
	loopingFile := getPropertyString("loop-file")
	//loopingPlaylist := getPropertyString("loop-playlist")
	if buffering != nil && buffering.(bool) {
		activity.SmallImageKey = "https://github.com/Fadexz/mpv-discord-improvedstatus/blob/main/assets/737663962677510245/buffer.png?raw=true"
		activity.SmallImageText = "Buffering"
	} else if pausing != nil && pausing.(bool) {
		activity.SmallImageKey = "player_pause"
		activity.SmallImageText = "Paused"
	} else if loopingFile != "no" /*|| loopingPlaylist != "no"*/ {
		activity.SmallImageKey = "https://github.com/Fadexz/mpv-discord-improvedstatus/blob/main/assets/737663962677510245/loop.png?raw=true"
		activity.SmallImageText = "Looping"
	} else {
		activity.SmallImageKey = "player_play"
		activity.SmallImageText = "Playing"
	}
	if percentage := getProperty("percent-pos"); percentage != nil {
		activity.SmallImageText += fmt.Sprintf(" (%d%%)", int(percentage.(float64)))
	}
	if pcount := getProperty("playlist-count"); pcount != nil && int(pcount.(float64)) > 1 {
		if ppos := getProperty("playlist-pos-1"); ppos != nil {
			activity.SmallImageText += fmt.Sprintf(" [%d/%d]", int(ppos.(float64)), int(pcount.(float64)))
		}
	}

	// Timestamps
	_duration := getProperty("duration")
	durationMillis := int64(_duration.(float64))
	_timePos := getProperty("time-pos")
	timePosMills := int64(_timePos.(float64))

	startTimePos := currTime - (timePosMills * 1000)
	duration := startTimePos + (durationMillis * 1000)

	if pausing != nil && !pausing.(bool) {
		activity.Timestamps = &discordrpc.ActivityTimestamps{
			Start: startTimePos,
			End:   duration,
		}
		refreshCurrTime()
	}
	return
}

func openClient() {
	if err := client.Open(os.Args[1]); err != nil {
		log.Fatalln(err)
	}
	log.Println("(mpv-ipc): connected")
}

func openPresence() {
	// try until success
	for range time.Tick(500 * time.Millisecond) {
		if client.IsClosed() {
			return // stop trying when mpv shuts down
		}
		if err := presence.Open(); err == nil {
			break // break when successfully opened
		}
	}
	log.Println("(discord-ipc): connected")
}

func main() {
	defer func() {
		if !client.IsClosed() {
			if err := client.Close(); err != nil {
				log.Fatalln(err)
			}
			log.Println("(mpv-ipc): disconnected")
		}
		if !presence.IsClosed() {
			if err := presence.Close(); err != nil {
				log.Fatalln(err)
			}
			log.Println("(discord-ipc): disconnected")
		}
	}()

	openClient()
	go openPresence()

	for range time.Tick(time.Second * 5) {
		activity, err := getActivity()
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				break
			} else if !errors.Is(err, io.EOF) {
				log.Println(err)
				continue
			}
		}
		if !presence.IsClosed() {
			go func() {
				if err = presence.Update(activity); err != nil {
					if errors.Is(err, syscall.EPIPE) {
						// close it before retrying
						if err = presence.Close(); err != nil {
							log.Fatalln(err)
						}
						log.Println("(discord-ipc): reconnecting...")
						go openPresence()
					} else if !errors.Is(err, io.EOF) {
						log.Println(err)
					}
				}
			}()
		}
	}
}
