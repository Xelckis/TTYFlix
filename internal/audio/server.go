package audio

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"charm.land/log/v2"
	"github.com/xelckis/ttyflix/internal/media"
)

type StreamSession struct {
	ID           string
	StartTrigger chan struct{}
	startOnce    sync.Once
	AudioBuffer  *media.RingBuffer[byte]
	totalBytes   int64
	PTS          int64
	mu           sync.RWMutex
	ctx          context.Context
}

func (s *StreamSession) TriggerStart() {
	s.startOnce.Do(func() {
		close(s.StartTrigger)
	})
}

func (s *StreamSession) AddBytes(bytesRead int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalBytes += int64(bytesRead)
	s.PTS = s.totalBytes / 16
}

func (s *StreamSession) GetPTS() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PTS
}

var (
	activeSessions = make(map[string]*StreamSession)
	sessionMutex   sync.RWMutex
)

func AddUser(id string, audioBuf *media.RingBuffer[byte], ctx context.Context) *StreamSession {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	s := &StreamSession{
		ID:           id,
		StartTrigger: make(chan struct{}),
		AudioBuffer:  audioBuf,
		ctx:          ctx,
	}
	activeSessions[id] = s
	return s
}

func GetSession(id string) (*StreamSession, bool) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	s, ok := activeSessions[id]
	return s, ok
}

func RemoveSession(id string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	delete(activeSessions, id)
}

func HttpServer() {
	mux := http.NewServeMux()

	mux.HandleFunc("/stream/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		session, ok := GetSession(id)
		if !ok {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}

		session.TriggerStart()

		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Cache-Control", "no-cache, no-store")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		chunk := make([]byte, 4096)
		reqCtx := r.Context()

		for {
			select {
			case <-reqCtx.Done():
				return
			case <-session.ctx.Done():
				return
			default:
			}

			n, err := session.AudioBuffer.ReadChunk(chunk)
			if err != nil {
				if err == io.EOF {
					return
				}
				time.Sleep(5 * time.Millisecond)
				continue
			}

			if n > 0 {
				if _, writeErr := w.Write(chunk[:n]); writeErr != nil {
					return
				}
				flusher.Flush()
				session.AddBytes(n)
			}
		}
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Info("Starting HTTP Audio server", "port", "8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("HTTP Audio server failed", "error", err)
	}
}
