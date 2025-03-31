package crypto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"golang.org/x/crypto/bcrypt"

	"gitlab.com/fcv-2025.net/internal/config"
	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/domain"
)

var _ primary.JWTService = (*JWTServiceImpl)(nil)

var (
	ErrInvalidToken = fmt.Errorf("invalid token")
)

type JWTServiceImpl struct {
	KeysByKeyTypeByResources map[primary.Resource]map[primary.ResourceKeyType]primary.ResourceKey
	KeyStoreFolder           string
	HMACSecretKey            string
}

func NewJWTService(jwtConfig *config.JwtConfig) primary.JWTService {
	return &JWTServiceImpl{
		KeyStoreFolder: jwtConfig.KeyVaultFolder,
		HMACSecretKey:  jwtConfig.Secret,
	}
}

func (J JWTServiceImpl) GenerateTokenRSAWithFile(ctx context.Context, data []byte, file string, method string) (string, error) {
	keyData, _ := os.ReadFile(fmt.Sprintf("%s/%s", J.KeyStoreFolder, file))
	key, _ := jwt.ParseRSAPrivateKeyFromPEM(keyData)

	var dataMap = make(map[string]interface{})

	err := json.NewDecoder(bytes.NewReader(data)).Decode(&dataMap)
	if err != nil {
		return "", err
	}

	//RS256
	tok := jwt.NewWithClaims(jwt.GetSigningMethod(method), jwt.MapClaims(dataMap))
	tok.Header["kid"] = file
	return tok.SignedString(key)
}

func (J JWTServiceImpl) VerifyTokenRSAWithFile(ctx context.Context, token string, file string, method string) (bool, error) {
	keyData, _ := os.ReadFile(fmt.Sprintf("%s/%s", J.KeyStoreFolder, file))
	key, _ := jwt.ParseRSAPublicKeyFromPEM(keyData)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false, ErrInvalidToken
	}
	sig, err := decodeSeg(parts[2])
	if err != nil {
		return false, err
	}
	methodByName := jwt.GetSigningMethod(method)
	if methodByName == nil {
		return false, fmt.Errorf("signing method %s not found", method)
	}

	err = methodByName.Verify(strings.Join(parts[0:2], "."), []byte(sig), key)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (J JWTServiceImpl) GenerateTokenRSA(ctx context.Context, privateKey string, method string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (J JWTServiceImpl) VerifyTokenRSA(ctx context.Context, token string, publicKey string, method string) (bool, error) {
	//TODO implement me
	panic("implement me")
}
func (J JWTServiceImpl) GenerateTokenHMAC(ctx context.Context, method string, claims map[string]interface{}) (string, error) {
	signingMethod := jwt.GetSigningMethod(method)
	if signingMethod == nil {
		return "", fmt.Errorf("unsupported signing method: %s", method)
	}

	// Ensure the claims map contains an expiration time
	if _, exists := claims["exp"]; !exists {
		claims["exp"] = time.Now().Add(time.Hour * 1).Unix() // Set expiry to 1 hour from now
	}

	tok := jwt.NewWithClaims(signingMethod, jwt.MapClaims(claims))
	return tok.SignedString([]byte(J.HMACSecretKey))
}

func (J JWTServiceImpl) VerifyTokenHMAC(ctx context.Context, token string, method string) (bool, error) {
	signingMethod := jwt.GetSigningMethod(method)
	if signingMethod == nil {
		return false, fmt.Errorf("unsupported signing method: %s", method)
	}

	parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(J.HMACSecretKey), nil
	})
	if err != nil {
		return false, err
	}

	return parsedToken.Valid, nil
}

func (JWTServiceImpl) VerifyPassword(ctx context.Context, passwordHash string, pwd string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(pwd))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (J JWTServiceImpl) EncryptPassword(ctx context.Context, password string) (string, error) {
	pwd, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(pwd), nil
}

func decodeSeg(signature string) (string, error) {
	sig, err := jwt.NewParser().DecodeSegment(signature)
	if err != nil {
		return "", err
	}
	return string(sig), nil
}

func (J JWTServiceImpl) DecodeTokenPayload(ctx context.Context, token string) (domain.AuthPayload, error) {
	// Decode the token without verifying it
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return domain.AuthPayload{}, fmt.Errorf("invalid token format")
	}

	// Decode the payload (second part of the token, base64-encoded)
	payloadData, err := decodeSeg(parts[1])
	if err != nil {
		return domain.AuthPayload{}, fmt.Errorf("failed to decode token payload: %w", err)
	}

	// Decrypt the payload into AuthPayload
	authPayload, err := J.DecryptAuthPayload([]byte(payloadData))
	if err != nil {
		return domain.AuthPayload{}, fmt.Errorf("failed to parse AuthPayload: %w", err)
	}

	return authPayload, nil
}

func (J JWTServiceImpl) DecryptAuthPayload(data []byte) (domain.AuthPayload, error) {
	var authPayload domain.AuthPayload

	// Unmarshal the JSON-encoded data into the AuthPayload struct
	err := json.Unmarshal(data, &authPayload)
	if err != nil {
		return domain.AuthPayload{}, fmt.Errorf("failed to decrypt AuthPayload: %w", err)
	}

	return authPayload, nil
}
