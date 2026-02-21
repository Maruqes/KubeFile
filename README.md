# KubeFile

A Kubernetes learning project demonstrating microservices architecture, file sharing, and URL shortening capabilities.

## Educational Purpose Only

**This project is designed exclusively for learning Kubernetes concepts and should NOT be used for production or any real-world applications.** It's a demonstration project to explore microservices, containerization, and Kubernetes orchestration.

## Project Overview

KubeFile is a distributed application that showcases:
- **Microservices Architecture**: Multiple services communicating via gRPC
- **File Sharing**: Chunked file upload/download with progress tracking
- **URL Shortening**: Simple URL shortener service
- **Storage Management**: Integration with MinIO for object storage
- **Real-time UI**: Modern web interface with progress indicators

## Architecture

The project consists of several microservices:

- **Gateway Service** (Port 8512): HTTP gateway and web interface
- **File Sharing Service** (Port 50052): Handles file operations and storage
- **URL Shortener Service** (Port 50051): Manages URL shortening
- **Redis**: Caching and session storage
- **MinIO**: Object storage for files

## 🛠️ Development Setup

### Prerequisites

- [Rancher Desktop](https://rancherdesktop.io/) - For local Kubernetes cluster
- [Tilt](https://tilt.dev/) - For development workflow automation
- Go 1.24+ - For building services
- kubectl - For Kubernetes management

### Local Development

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd KubeFile
   ```

2. **Start Rancher Desktop**
   - Install and start Rancher Desktop
   - Enable Kubernetes support
   - Ensure kubectl is configured to use the local cluster

3. **Set up environment**
   ```bash
   # Create .env file with your K8s context
   echo "K8_CONTEXT=rancher-desktop" > .env
   ```

4. **Start development with Tilt**
   ```bash
   tilt up
   ```

5. **Access the application**
   - Web Interface: http://localhost:8512
   - Tilt Dashboard: http://localhost:10350

### Development Features

- **Live Reload**: Code changes automatically trigger rebuilds
- **Fast Builds**: Optimized Docker builds with caching
- **Port Forwarding**: Direct access to all services
- **Log Streaming**: Real-time logs from all services

## Production Deployment

The production environment runs on **k3s** (lightweight Kubernetes) on an isolated virtual machine. This setup demonstrates:

- Container orchestration in a real Kubernetes environment
- Service discovery and networking
- Persistent storage management
- Load balancing and scaling

## Project Structure

```
KubeFile/
├── services/           # Microservices source code
│   ├── gateway/       # HTTP gateway & web UI
│   ├── filesharing/   # File operations service
│   └── shortener/     # URL shortening service
├── k8s/               # Kubernetes manifests
├── shared/proto/      # gRPC protocol definitions
├── Tiltfile          # Development automation
└── go.mod            # Go module definition
```

## Learning Objectives

This project demonstrates:

## Technologies Used

- **Go**: Backend services
- **gRPC**: Service communication
- **Kubernetes**: Container orchestration
- **Docker**: Containerization
- **MinIO**: Object storage
- **Redis**: Caching layer
- **HTML/CSS/JavaScript**: Frontend interface
- **Tilt**: Development automation
- **Rancher Desktop**: Local Kubernetes

## Features

- **File Upload/Download**: Chunked file transfer with progress tracking
- **URL Shortening**: Create and resolve short URLs
- **Storage Monitoring**: Real-time storage usage display
- **Responsive UI**: Modern web interface
- **Progress Indicators**: Visual feedback for long operations
- **Error Handling**: Robust error management and retry logic

## Important Notes

- **Security**: This is a learning project with minimal security measures
- **Scalability**: Designed for demonstration, not high-load scenarios
- **Data Persistence**: Data may be lost during development iterations

## License

This project is provided as-is for educational purposes. See LICENSE file for details.

---

**Remember: This is a learning project. Do not use it for production workloads or store sensitive data.**
