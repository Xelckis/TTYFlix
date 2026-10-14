# TTYFlix 🎬🍿

> **Terminal Cinema:** Stream and watch videos directly in your terminal over SSH with synchronized audio!

TTYFlix is an interactive terminal-based video streaming server written in Go. It turns your terminal into a cinema screen by rendering video frames in 24-bit TrueColor ANSI graphics using half-block characters while streaming synchronized MP3 audio over HTTP.

---

## ✨ Features

- **📺 Terminal Video Rendering:**
  - High-fidelity **24-bit TrueColor** (RGB) rendering in any modern terminal.
  - **Half-block character technique (`▀`)** packing two vertical pixels per character cell to double vertical resolution.
  - **Delta Frame Rendering:** Only modified screen cells are redrawn between frames, dramatically reducing terminal I/O and network bandwidth.
  - **Run-Length Encoding (RLE):** Batches identical contiguous color blocks into single ANSI escape sequences.

- **🔊 Synchronized Audio Streaming:**
  - Integrated HTTP chunked MP3 streaming server.
  - **Audio-Master PTS Synchronization:** Uses audio transmission progress as the master clock to synchronize video playback and automatically drop frames when lagging.
  - Playback synchronizes automatically the moment the client connects to the audio stream.

- **⚡ Interactive SSH & TUI:**
  - Native SSH server powered by [Charm Wish](https://github.com/charmbracelet/wish) and [Bubble Tea](https://github.com/charmbracelet/bubbletea).
  - Clean interactive TUI catalog to browse and select movies/videos located in the `./media` directory.
  - Built-in SSH port forwarding support (`-L 8080:localhost:8080`) for seamless remote audio listening.

---

## 🛠️ Requirements

### Server Requirements
- **Go:** `1.22+` (or `1.27+`)
- **FFmpeg:** Must be installed and available in `$PATH` (with `libmp3lame` support).

### Client Requirements
- **SSH Client:** Standard OpenSSH client (`ssh`).
- **Terminal Emulator:** Any terminal with 24-bit TrueColor and UTF-8 support (e.g., Alacritty, Kitty, WezTerm, iTerm2, modern Windows Terminal, GNOME Terminal).
- **Audio Player:** An audio player capable of streaming HTTP MP3 streams, such as `ffplay` (bundled with FFmpeg), `mpv`, or `vlc`.

---

## 🚀 Getting Started

### 1. Clone & Build

```bash
git clone https://github.com/xelckis/ttyflix.git
cd ttyflix
go build -o ttyflix main.go
```

### 2. Add Media

Place any `.mp4` video files you want to stream into the `./media` folder:

```bash
mkdir -p media
cp /path/to/your/video.mp4 media/
```

### 3. Start the Server

```bash
./ttyflix
```

By default:
- **SSH Server** listens on `0.0.0.0:2222` (Host key automatically stored at `.ssh/ttyflix_id_ed25519`).
- **HTTP Audio Server** listens on `0.0.0.0:8080`.

---

## 📖 How to Use

### Step 1: Connect via SSH

Connect to the TTYFlix server using your terminal. If you are connecting remotely, forward port `8080` to access the audio stream locally:

```bash
ssh -p 2222 -L 8080:localhost:8080 live@localhost
```
*(Replace `localhost` with your remote server's IP or hostname if running remotely).*

### Step 2: Choose a Video

Use the arrow keys or `j`/`k` to navigate the catalog, `/` to filter, and press **Enter** to choose a video.

### Step 3: Start Audio Playback

Once a video is selected, TTYFlix will display a waiting screen with a unique streaming URL command:

```bash
ffplay -nodisp -autoexit http://localhost:8080/stream/<SESSION_UUID>
```

Open a second terminal window (or run in background) and paste that command. You can also use `mpv`:

```bash
mpv --no-video http://localhost:8080/stream/<SESSION_UUID>
```

As soon as the audio player connects, video playback will start immediately in your main SSH terminal in perfect sync!

### Controls

- **`q`** or **`Ctrl+C`**: Stop playback and exit.

---

## 🏗️ Architecture & How It Works

```
                     ┌──────────────────────────────────────────────┐
                     │                   TTYFlix                    │
                     │                                              │
                     │   ┌───────────────┐     ┌────────────────┐   │
                     │   │   SSH Server  │     │   HTTP Server  │   │
                     │   │  (Wish / TUI) │     │    (:8080)     │   │
                     │   └──────┬────────┘     └───────┬────────┘   │
                     │          │                      │            │
                     │          │ Video Deltas (ANSI)  │ MP3 Stream │
                     └──────────┼──────────────────────┼────────────┘
                                │                      │
                        ┌───────▼──────┐        ┌──────▼───────┐
                        │  Terminal    │        │ Audio Player │
                        │ (SSH Client) │        │ (ffplay/mpv) │
                        └──────────────┘        └──────────────┘
```

1. **SSH Connection & Catalog Selection:**  
   When a client connects over SSH, Charm's Wish & Bubbletea display the interactive file selector.
2. **Session Initialization:**  
   A unique UUID is generated per session. In-memory generic ring buffers are allocated for both raw video RGB frames and encoded MP3 audio chunks.
3. **FFmpeg Engine:**  
   FFmpeg runs as a background process per session, scaling video to match terminal width & height (`scale=width:height*2`) at 30 FPS in raw RGB24, while simultaneously encoding audio to 128 kbps MP3.
4. **AV Synchronization:**  
   The HTTP server tracks bytes streamed to calculate Presentation Time Stamp (PTS = `totalBytes / 16`). The video rendering loop compares the current video frame against master audio PTS, dropping lagging frames or pacing output accordingly.
5. **Delta Frame Rendering:**  
   Frame differences are identified cell-by-cell and rendered using the half-block (`▀`) character with 24-bit ANSI foreground (top pixel) and background (bottom pixel) color codes, with RLE compaction.

---

## ⚙️ Configuration

The default network configuration can be customized in `internal/ssh/ssh.go` and `internal/audio/server.go`:

| Component | Default Address / Port | Description |
|---|---|---|
| **SSH Server** | `0.0.0.0:2222` | SSH terminal connection and TUI interface |
| **HTTP Audio** | `0.0.0.0:8080` | Audio stream endpoint (`/stream/{uuid}`) |
| **Media Directory** | `./media` | Directory where `.mp4` files are scanned |

---

## 📄 License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.
