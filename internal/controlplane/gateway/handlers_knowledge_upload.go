package gateway

// Knowledge upload moved to internal/packs/knowledge. This file intentionally
// stays as a marker for the migration boundary: Gateway still owns generic file
// upload (/v1/upload), while /v1/knowledge/upload is a native knowledge pack
// route.
