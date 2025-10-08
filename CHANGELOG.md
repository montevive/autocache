# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-10-08

### Added

#### Core Features
- Intelligent cache-control injection for Anthropic Claude API requests
- Automatic token counting and analysis for optimal cache breakpoint placement
- ROI analytics with detailed cost savings calculations
- Support for multiple cache strategies (conservative, moderate, aggressive)
- Streaming and non-streaming request support
- Response headers with detailed cache metadata (X-Autocache-* headers)

#### Cache Intelligence
- Smart breakpoint placement using ROI scoring algorithm
- Token minimums enforcement (1024 for most models, 2048 for Haiku)
- Automatic TTL assignment (1h for stable content, 5m for dynamic)
- Support for system prompts, tool definitions, and content blocks

#### API Endpoints
- `POST /v1/messages` - Main proxy endpoint with automatic caching
- `GET /health` - Health check endpoint
- `GET /metrics` - Metrics and configuration endpoint
- `GET /savings` - Comprehensive savings analytics and statistics

#### Configuration
- Environment variable configuration support
- Multiple caching strategies with customizable thresholds
- API key handling via headers or environment variables
- Configurable logging (text/JSON, multiple levels)

#### Docker Support
- Multi-stage Dockerfile with optimized build
- Docker Compose configuration with health checks
- Non-root user security
- Resource limits and restart policies

#### Documentation
- Comprehensive README with examples
- Architecture documentation (CLAUDE.md)
- API key handling guide
- n8n integration documentation
- Troubleshooting guides

#### Testing
- Unit tests for all core components
- Real API integration tests
- Test fixtures and utilities
- n8n workflow testing scripts

### Technical Details
- Built with Go 1.23+
- Modular architecture with clean separation of concerns
- Logrus-based structured logging
- Graceful shutdown handling
- HTTPS support for Anthropic API communication

[1.0.0]: https://github.com/montevive/autocache/releases/tag/v1.0.0
