# Go Port Transfer Service

High-performance port forwarding and device management service with unified HTTP/WebSocket API support.

## Overview

This service provides a modular backend for managing device connections and port mappings. It features a unified API architecture where all endpoints are accessible via both HTTP (POST) and WebSocket, ensuring consistent data structures across protocols.

## Features

- **Unified API Architecture**: All business logic is exposed via identical Request/Response structures for both HTTP and WebSocket.
- **Interactive Documentation**: Auto-generated, visual API documentation available at `/doc`.
- **Dual Protocol Support**:
  - **HTTP API**: Listen on port `1001` (POST only).
  - **WebSocket**: Listen on port `1002`.
- **Strict Typing**: Request/Response models are strictly defined and validated using Go generics.
- **Docker Ready**: Optimized multi-stage Docker build process.

## Quick Start

### Prerequisites

- Go 1.25+
- Docker (optional)

### Local Development

1. **Build the binary**
   ```bash
   go build -o server main.go
   ```

2. **Run the service**
   ```bash
   ./server
   ```
   
   The service will start:
   - HTTP Server: `http://0.0.0.0:1001`
   - WebSocket Server: `ws://0.0.0.0:1002`

3. **View Documentation**
   Open `http://localhost:1001/doc` in your browser.

### Docker Deployment

**Option 1: Using Docker Compose (Recommended)**

This is the simplest way to build and start the service.

```bash
docker compose up -d --build
```

**Option 2: Manual Build & Run**

1. **Build Image**
   ```bash
   docker build -t go-port-trans .
   ```

2. **Run Container**
   ```bash
   docker run -d \
     -p 1001:1001 \
     -p 1002:1002 \
     --name port-trans \
     go-port-trans
   ```

   *Note: If you encounter an error saying `image 'go-port-trans:latest' not found`, ensure you have executed the build step (Step 1) successfully.*

## API Documentation & Usage

The project includes a self-hosted documentation page.
Visit `/doc` endpoint (e.g., `http://localhost:1001/doc`) to:
- View all available endpoints.
- Check Request/Response JSON schemas.
- **One-Click Copy**: Copy the full API definition context for AI assistants.

### WebSocket Protocol

The WebSocket interface uses a simple envelope protocol to route requests to the same handlers as the HTTP API.

**Connection**: `ws://<host>:1002`

**Request Envelope**:
```json
{
  "path": "/api/connect",      // Matches the HTTP route path
  "id": "unique-req-id",       // Optional correlation ID
  "data": {                    // The actual request payload (same as HTTP POST body)
    "key": "...",
    "deviceId": 123
  }
}
```

**Response Envelope**:
```json
{
  "id": "unique-req-id",       // Echoes the request ID
  "path": "/api/connect",
  "success": true,             // Execution status
  "message": "",               // Error message if success is false
  "data": { ... }              // Response payload
}
```

## Project Structure

```
.
├── Dockerfile              # Docker build configuration
├── main.go                 # Application entry point & route registration
├── run.sh                  # Startup script
├── internal/
│   ├── manager/            # State management (Singleton)
│   ├── model/              # Request/Response struct definitions
│   └── service/            # Business logic implementation
└── pkg/
    ├── framework/          # Web/WS framework & auto-doc engine
    └── portmap/            # Core port forwarding logic
```
