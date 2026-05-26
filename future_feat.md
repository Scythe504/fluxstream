# Fluxstream Core Video Engine - Future Roadmap & Features

This document outlines key technical enhancements and features to elevate the `fluxstream` core streaming engine to a premium media-player experience.

---

## 🌟 High Priority

### 1. Multi-Language Subtitle Tracks Selection
Currently, the subtitle handler automatically extracts the first subtitle stream (`0:s:0`) via FFmpeg. Many files contain multiple subtitle tracks (e.g. Dialogue, Signs & Songs, different language translations).
- **Backend implementation**: Use `ffprobe` to fetch all available subtitle streams, return their language codes and titles to the frontend, and allow the `serveSubtitles` handler to accept a track index parameter (e.g., `/videos/{videoId}/subs?index=2`) which maps to `-map 0:s:{index}` in FFmpeg.
- **Frontend implementation**: Display a language-selection menu inside the subtitle button popover.

---

## 🚀 Medium Priority

### 2. Dual-Audio Selector (Language Dubs)
Raw MKV files frequently carry dual-audio tracks (e.g. Japanese original and English dubs). Since HTML5 `<video>` tags cannot switch raw audio tracks natively:
- **Solution**: Implement a dynamic, lightweight backend HLS (HTTP Live Streaming) transcoder that repackages the torrent stream in real-time, serving selectable audio track options to the client.

### 3. Watch Progress & Resume Playback
Integrate watch history to remember where the user left off.
- **Backend/DB**: Store user watch progression states (`video_id`, `last_position_seconds`, `completed`, `updated_at`).
- **Frontend**: Periodically ping watch progress to the backend, and present a **"Resume Playback"** prompt when the user restarts a video.

### 4. Playback Speed Selector
- **Implementation**: Add standard speed control selections (`0.25x`, `0.5x`, `1.0x`, `1.25x`, `1.5x`, `2.0x`) within the video player controls overlay.

---

## 💎 Advanced Features

### 5. Smart Pre-fetching & Instant Episode Transitions
Reduce playback startup delays.
- **Sequential Buffer Tuning**: Implement dynamic torrent piece-prioritization based on playback speed and seeking.
- **Pre-fetch Next Episode**: When the user reaches the final 5 minutes of an episode, command the backend torrent engine to pre-fetch the metadata and first 5% of the next episode in the background.

### 6. Picture-in-Picture (PiP) & Picture Overlay
- **Implementation**: Add a control button using the browser's native `document.pictureInPictureEnabled` API to let users watch episodes in a draggable overlay while browsing.

### 7. Skip Intro / Skip Outro (AniSkip Integration)
- **Implementation**: Integrate with public databases like [AniSkip](https://github.com/aniskip) to dynamically show **"Skip Intro"** or **"Skip Outro"** overlay buttons at exact anime timestamp ranges.

---

## 🔮 V3.0 (Long-Term Roadmap)

### 8. Stremio Addon Compatibility Layer (Plugin System)
Directly support Stremio’s standard HTTP JSON addon protocol to allow infinite catalog expansion and scraping capabilities:
- **Manifest Parser & DB Loader**: Allow users to paste a Stremio manifest URL (e.g. `/manifest.json`) in the settings UI. Parse and store capabilities and metadata dynamically.
- **Protocol Translation Layer**: Build a backend adapter to map Stremio's API responses (e.g., `/catalog`, `/meta`, `/stream`) to Fluxstream's internal `Media`, `Episode`, and `Source` interfaces.
- **Debrid Provider Integration (Real-Debrid, Premiumize)**: Add support for link-unrestricting APIs so users can safely stream cached torrent links over high-speed HTTPS instead of using public BitTorrent swarms.
- **Self-Hosting Options**: Provide options for users to spin up local instances of popular open-source Stremio addons directly in Fluxstream to avoid rate-limiting or server load on public addon instances.
