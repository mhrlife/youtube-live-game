# Headless Game Streamer

A fun project that runs a headless game on Linux servers, streaming gameplay live to YouTube (or Twitch) and allowing user interactions via chat to influence the game in real-time.

## Features

- **Headless Gameplay:** Runs entirely on Linux servers without a graphical interface.
- **Live Streaming:** Streams gameplay using FFMPEG to platforms like YouTube Live or Twitch.
- **Interactive Chat:** Listens to live chat for user inputs to dynamically update the game.

## How It Works

1. **Streaming Setup:**
   - Launches an FFMPEG instance compatible with your chosen live platform.
   - Generates each game frame and streams it directly to the FFMPEG instance.

2. **User Interaction:**
   - Runs a separate goroutine to listen to live chat.
   - Processes user inputs from chat to update and influence the game in real-time.

## Getting Started

### Prerequisites

- [Docker](https://www.docker.com/get-started) installed on your machine.
- [Docker Compose](https://docs.docker.com/compose/install/) for managing multi-container Docker applications.

### Running Locally

1. **Start Nginx RTMP Server:**
   ```bash
   docker compose -f docker-compose.nginx-rtmp.yml up -d
   ```

2. **Run the Game Streamer:**
   ```bash
   make local
   ```

   This will start the headless game and begin streaming to the local Nginx RTMP server.

### Deploying to a Server

1. **Build the Docker Image:**
   ```bash
   docker build -t your-username/headless-game-streamer .
   ```

2. **Push to Docker Registry:**
   ```bash
   docker push your-username/headless-game-streamer
   ```

3. **Run the Container:**
   ```bash
   docker run -e STREAM_URL=<YOUR_STREAM_URL_WITH_TOKEN> your-username/headless-game-streamer
   ```

    - Replace `<YOUR_STREAM_URL_WITH_TOKEN>` with your streaming platform's URL including the stream key/token.
    - Supports platforms like YouTube Live and Twitch.

## Configuration

- **STREAM_URL:** Environment variable to set your streaming destination. Format typically looks like `rtmp://live.youtube.com/app/<STREAM_KEY>` or `rtmp://live.twitch.tv/app/<STREAM_KEY>`.

## License

This project is licensed under the [MIT License](LICENSE).
