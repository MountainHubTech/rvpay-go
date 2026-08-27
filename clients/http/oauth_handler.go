package http

import (
	"net/http"

	"github.com/I-Frostbyte/rvpay-go/clients/oauth"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OAuthHandler exposes the OAuth callback endpoint for provider redirects.
type OAuthHandler struct {
	service *oauth.Service
	logger  zerolog.Logger
}

// NewOAuthHandler creates a new OAuth HTTP handler.
func NewOAuthHandler(service *oauth.Service, logger zerolog.Logger) *OAuthHandler {
	return &OAuthHandler{
		service: service,
		logger:  logger,
	}
}

// Callback handles the OAuth authorization callback from a provider.
//
// Route: GET /oauth/callback
// Query parameters: code, state
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	h.logger.Info().Msg("Callback method engaged...")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info().Msg("Handler reached ")

	code := r.URL.Query().Get("code")
	// state := r.URL.Query().Get("state")

	var state string

	// Parsing the query parameters into a map (url.Values)
	queryParams := r.URL.Query()

	// Check if the "state" key exists in the map at all
	if _, exists := queryParams["state"]; !exists {
		// Scenario A: The parameter is completely missing from the URL
		// Example: /callback?code=123
		state = ""
		h.logger.Info().Msg("State parameter doesn't exist, given empty string value.")
	} else {
		// Scenario B & C: Key exists, now fetch its value
		// Example: /callback?code=123&state=
		state = queryParams.Get("state")
		if state == "" {
			h.logger.Info().Msg("State parameter exists but has no value.")
		}
	}

	result, err := h.service.HandleCallback(r.Context(), code, state)
	if err != nil {
		h.writeCallbackError(w, err)
		return
	}

	h.logger.Info().
		Str("integration_id", result.IntegrationID.String()).
		Str("client_id", result.ClientID.String()).
		Str("platform_id", result.PlatformID.String()).
		Msg("OAuth callback handled successfully")

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("integration authorized successfully"))
}

// writeCallbackError maps domain errors to appropriate HTTP status codes
// without exposing internal details, tokens, or secrets.
func (h *OAuthHandler) writeCallbackError(w http.ResponseWriter, err error) {
	code := status.Code(err)

	switch code {
	case codes.InvalidArgument:
		http.Error(w, "invalid OAuth callback parameters", http.StatusBadRequest)
	case codes.NotFound:
		http.Error(w, "OAuth context not found", http.StatusBadRequest)
	case codes.FailedPrecondition:
		http.Error(w, "OAuth flow not permitted", http.StatusBadRequest)
	case codes.AlreadyExists:
		http.Error(w, "integration already exists", http.StatusConflict)
	default:
		h.logger.Error().Err(err).Msg("OAuth callback processing failed")
		http.Error(w, "OAuth callback processing failed", http.StatusInternalServerError)
	}
}
