package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

func RunFFmpeg(ctx context.Context, inputFile string, width int, height int, audioRb *RingBuffer[byte], videoRb *RingBuffer[byte]) error {
	audioReader, audioWriter, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("could not create audio pipe: %w", err)
	}

	args := []string{
		"-re",
		"-i", inputFile,
		"-map", "0:v:0",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height*2),
		"-r", "30",
		"-c:v", "rawvideo",
		"-pix_fmt", "rgb24",
		"-f", "rawvideo",
		"pipe:1",
		"-map", "0:a:0?",
		"-c:a", "libmp3lame",
		"-b:a", "128k",
		"-ar", "44100",
		"-ac", "2",
		"-f", "mp3",
		"pipe:3",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.ExtraFiles = []*os.File{audioWriter}

	videoReader, err := cmd.StdoutPipe()
	if err != nil {
		audioWriter.Close()
		audioReader.Close()
		return fmt.Errorf("could not create stdout pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		audioWriter.Close()
		audioReader.Close()
		return fmt.Errorf("could not start ffmpeg: %w", err)
	}

	audioWriter.Close()

	go func() {
		defer videoRb.Close()
		buf := make([]byte, 65536)
		for {
			n, err := videoReader.Read(buf)
			if n > 0 {
				videoRb.WriteChunk(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		defer audioReader.Close()
		defer audioRb.Close()
		buf := make([]byte, 4096)
		for {
			n, err := audioReader.Read(buf)
			if n > 0 {
				audioRb.WriteChunk(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		_ = cmd.Wait()
		videoRb.Close()
		audioRb.Close()
	}()

	return nil
}
