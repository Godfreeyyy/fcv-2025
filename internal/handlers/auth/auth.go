package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"gitlab.com/fcv-2025.net/internal/core/services/auth"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/handlers/response"
)

type ServiceDependencies struct {
	GGAuthService    auth.IAuthService
	LocalAuthService auth.IAuthService
}

var googleOAuthConfig = &oauth2.Config{
	ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	RedirectURL:  "http://localhost:8082/auth/callback",
	Scopes:       []string{"profile", "email"},
	Endpoint:     google.Endpoint,
}

// GoogleUser struct to decode Google API response
type GoogleUser struct {
	ID    string `json:"sub"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Handler struct {
	providerHandler map[domain.Provider]auth.IAuthService
}

func NewHandler() *Handler {
	return &Handler{
		providerHandler: make(map[domain.Provider]auth.IAuthService),
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router, svcDep *ServiceDependencies) {
	h.providerHandler[domain.ProviderGoogle] = svcDep.GGAuthService
	h.providerHandler[domain.ProviderLocal] = svcDep.LocalAuthService
	router.HandleFunc("/auth/google", GoogleLoginHandler)
	router.HandleFunc("/auth/callback", h.GoogleCallbackHandler).Methods("GET")
}

// GoogleLoginHandler redirects user to Google OAuth2 login
func GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	url := googleOAuthConfig.AuthCodeURL("randomstate")
	fmt.Println(url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GoogleCallbackHandler handles Google OAuth2 callback
func (h *Handler) GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	fmt.Println("GoogleCallbackHandler")
	// Get authorization code from URL
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code in URL", http.StatusBadRequest)
		return
	}
	// Exchange code for access token
	token, err := googleOAuthConfig.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to get token", http.StatusInternalServerError)
		return
	}
	// Fetch user info from Google API
	client := googleOAuthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	// Decode Google user info
	var googleUser GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		http.Error(w, "Failed to decode user info", http.StatusInternalServerError)
		return
	}
	// Return JWT to client
	w.Header().Set("Content-Type", "application/json")
	tokenStr, err := h.providerHandler[domain.ProviderGoogle].Login(ctx, &domain.Users{
		GoogleID:     &googleUser.ID,
		Email:        &googleUser.Email,
		AuthProvider: string(domain.ProviderGoogle),
	})
	if err != nil {
		response.WriteError(w, response.ErrorMessage{
			Message:    err.Error(),
			StatusCode: http.StatusUnauthorized,
		})
		return
	}

	loginResponse := domain.LoginResponse{
		Token: tokenStr,
	}
	response.WriteSuccess(w, loginResponse)
}
