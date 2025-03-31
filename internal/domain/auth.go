package domain

type Provider string

const (
	ProviderGoogle Provider = "google"
	ProviderGithub Provider = "github"
	ProviderLocal  Provider = "local"
)

type AuthPayload struct {
	Username   string          `json:"username"`
	Permission []string        `json:"permission"`
	Resources  map[string]bool `json:"features"`
}

type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}
