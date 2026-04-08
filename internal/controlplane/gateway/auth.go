package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/apperror"
)

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret     string
	Issuer     string
	Expiration time.Duration
}

// jwtHeader is the JWT header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// jwtClaims is the JWT payload.
type jwtClaims struct {
	Sub      string `json:"sub"` // tenant ID
	Iss      string `json:"iss"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"` // "admin", "user"
}

// GenerateJWT creates a signed JWT token for a tenant.
func GenerateJWT(cfg JWTConfig, tenantID, role string) (string, error) {
	header := jwtHeader{Alg: "HS256", Typ: "JWT"}
	now := time.Now()
	claims := jwtClaims{
		Sub:      tenantID,
		Iss:      cfg.Issuer,
		Iat:      now.Unix(),
		Exp:      now.Add(cfg.Expiration).Unix(),
		TenantID: tenantID,
		Role:     role,
	}

	hJSON, _ := json.Marshal(header)
	cJSON, _ := json.Marshal(claims)

	hEnc := base64URLEncode(hJSON)
	cEnc := base64URLEncode(cJSON)
	sigInput := hEnc + "." + cEnc

	mac := hmac.New(sha256.New, []byte(cfg.Secret))
	mac.Write([]byte(sigInput))
	sig := base64URLEncode(mac.Sum(nil))

	return sigInput + "." + sig, nil
}

// ValidateJWT verifies a JWT token and returns claims.
func ValidateJWT(cfg JWTConfig, token string) (*jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errInvalidToken
	}

	// Verify signature
	mac := hmac.New(sha256.New, []byte(cfg.Secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64URLEncode(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errInvalidSignature
	}

	// Decode claims
	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, errInvalidToken
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, errInvalidToken
	}

	// Check expiration
	if time.Now().Unix() > claims.Exp {
		return nil, errTokenExpired
	}

	return &claims, nil
}

type authError string

func (e authError) Error() string { return string(e) }

const (
	errInvalidToken     authError = "invalid token"
	errInvalidSignature authError = "invalid signature"
	errTokenExpired     authError = "token expired"
)

// requireAuthJWT is a middleware that supports both API Key and JWT.
func (g *Gateway) requireAuthJWT(jwtCfg *JWTConfig, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := authTokenFromHeaders(r)
		if token == "" {
			apperror.WriteCode(w, apperror.CodeUnauthorized, "invalid or missing credentials")
			return
		}

		if t := g.tenants.ByAPIKey(token); t != nil {
			ctx := context.WithValue(r.Context(), ctxTenantKey, t.ID)
			next(w, r.WithContext(ctx))
			return
		}

		if jwtCfg != nil {
			claims, err := ValidateJWT(*jwtCfg, token)
			if err == nil {
				ctx := context.WithValue(r.Context(), ctxTenantKey, claims.TenantID)
				ctx = context.WithValue(ctx, ctxRoleKey, claims.Role)
				next(w, r.WithContext(ctx))
				return
			}
		}
		apperror.WriteCode(w, apperror.CodeUnauthorized, "invalid or missing credentials")
	}
}

func authTokenFromHeaders(r *http.Request) string {
	token := r.Header.Get("X-API-Key")
	if token == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return strings.TrimSpace(token)
}

func authTokenFromQuery(r *http.Request) string {
	q := r.URL.Query()
	for _, key := range []string{"key", "api_key", "token", "access_token"} {
		if v := strings.TrimSpace(q.Get(key)); v != "" {
			return v
		}
	}
	return ""
}

const ctxRoleKey ctxKeyType = "role"

func roleFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRoleKey).(string); ok {
		return v
	}
	return "user"
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func base64URLDecode(s string) ([]byte, error) {
	// Add padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}
