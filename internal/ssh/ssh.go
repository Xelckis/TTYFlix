package ssh

import (
	"bytes"
	"context"
	"errors"
	"net"
	"path/filepath"
	"time"

	"charm.land/log/v2"
	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
	"github.com/google/uuid"
	"github.com/xelckis/ttyflix/internal/audio"
	"github.com/xelckis/ttyflix/internal/media"
	"github.com/xelckis/ttyflix/internal/tui"
)

const (
	host = "0.0.0.0"
	port = "2222"
)

func withPortForwarding() ssh.Option {
	return func(s *ssh.Server) error {
		s.LocalPortForwardingCallback = func(ctx ssh.Context, dhost string, dport uint32) bool {
			return true
		}
		if s.ChannelHandlers == nil {
			s.ChannelHandlers = make(map[string]ssh.ChannelHandler)
		}
		s.ChannelHandlers["session"] = ssh.DefaultSessionHandler
		s.ChannelHandlers["direct-tcpip"] = ssh.DirectTCPIPHandler
		return nil
	}
}

func setUUID(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		userUUID := uuid.New().String()
		sess.Context().SetValue("uuid", userUUID)
		next(sess)
	}
}

func motorHandler(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		selectedVideo, ok := sess.Context().Value("selected_video").(string)
		if !ok || selectedVideo == "" {
			wish.Println(sess, "\r\nNenhum vídeo selecionado. Sessão finalizada. Até logo!")
			next(sess)
			return
		}

		userUUID, _ := sess.Context().Value("uuid").(string)

		// Display Wait Screen
		videoTitle := filepath.Base(selectedVideo)
		if len(videoTitle) > 50 {
			videoTitle = videoTitle[:47] + "..."
		}

		wish.Print(sess, "\033[2J\033[H")
		wish.Println(sess, "╔════════════════════════════════════════════════════════════════════════════════╗")
		wish.Println(sess, "║                             TTYFlix - Terminal Cinema                          ║")
		wish.Println(sess, "╠════════════════════════════════════════════════════════════════════════════════╣")
		wish.Printf(sess, "║ Vídeo Selecionado: %-59s ║\r\n", videoTitle)
		wish.Println(sess, "║                                                                                ║")
		wish.Println(sess, "║ Abra outro terminal (ou use o túnel SSH) e execute o comando para ouvir:       ║")
		wish.Println(sess, "║                                                                                ║")
		wish.Printf(sess, "║   ffplay -nodisp -autoexit http://localhost:8080/stream/%s ║\r\n", userUUID)
		wish.Println(sess, "║                                                                                ║")
		wish.Println(sess, "║ Aguardando conexão de áudio no ffplay para iniciar sincronizado...             ║")
		wish.Println(sess, "║ (Pressione Ctrl+C ou 'q' a qualquer momento para sair)                         ║")
		wish.Println(sess, "╚════════════════════════════════════════════════════════════════════════════════╝")

		ctx, cancel := context.WithCancel(sess.Context())
		defer cancel()

		audioBuf := media.NewRingBuffer[byte](4 * 1024 * 1024)
		videoBuf := media.NewRingBuffer[byte](8 * 1024 * 1024)

		audioSession := audio.AddUser(userUUID, audioBuf, ctx)
		defer audio.RemoveSession(userUUID)
		defer audioBuf.Close()
		defer videoBuf.Close()

		go func() {
			buf := make([]byte, 16)
			for {
				n, err := sess.Read(buf)
				if err != nil {
					cancel()
					return
				}
				for i := range n {
					if buf[i] == 'q' || buf[i] == 'Q' || buf[i] == 3 /* Ctrl+C */ {
						cancel()
						return
					}
				}
			}
		}()

		select {
		case <-ctx.Done():
			wish.Println(sess, "\r\nReprodução cancelada.")
			next(sess)
			return
		case <-audioSession.StartTrigger:
		}

		pty, _, _ := sess.Pty()
		width := pty.Window.Width
		height := pty.Window.Height
		if width <= 0 {
			width = 80
		}
		if height <= 0 {
			height = 24
		}

		err := media.RunFFmpeg(ctx, selectedVideo, width, height, audioBuf, videoBuf)
		if err != nil {
			wish.Printf(sess, "\r\nErro ao iniciar motor FFmpeg: %v\r\n", err)
			next(sess)
			return
		}

		wish.Print(sess, "\033[?1049h\033[?25l\033[2J\033[H")
		defer wish.Print(sess, "\033[?25h\033[?1049l\033[0m")

		fps := 30
		frameDurationMs := int64(1000 / fps) // ~33ms
		currentFrameTime := int64(0)

		frameSize := width * (height * 2) * 3
		frameData := make([]byte, frameSize)

		frameState := media.NewFrameState(width, height)
		ticker := time.NewTicker(time.Second / time.Duration(fps))
		defer ticker.Stop()

		var netBuf bytes.Buffer
		netBuf.Grow(32768)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			masterPTS := audioSession.GetPTS()

			dropped := false
			for currentFrameTime < masterPTS-frameDurationMs {
				if err := videoBuf.DiscardChunk(frameSize); err != nil {
					break
				}
				currentFrameTime += frameDurationMs
				dropped = true
			}
			if dropped {
				frameState.FirstFrame = true
			}

			if currentFrameTime > masterPTS+frameDurationMs {
				continue
			}

			err := videoBuf.ReadExact(frameData)
			if err != nil {
				// Video reached EOF or closed
				break
			}

			netBuf.Reset()
			frameState.RenderDeltas(frameData, &netBuf)

			if netBuf.Len() > 0 {
				if _, err := sess.Write(netBuf.Bytes()); err != nil {
					return
				}
			}

			currentFrameTime += frameDurationMs
		}

		wish.Println(sess, "\r\n\r\nReprodução concluída. Obrigado por usar o TTYFlix!")
		next(sess)
	}
}

func InitServer() {
	srv, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/ttyflix_id_ed25519"),
		withPortForwarding(),
		wish.WithMiddleware(
			motorHandler,
			bubbletea.Middleware(tui.TeaHandler),
			setUUID,
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not configure SSH server", "error", err)
		return
	}
	defer srv.Close()

	log.Info("Starting TTYFlix SSH server", "host", host, "port", port)
	if err = srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("SSH server error", "error", err)
	}
}
