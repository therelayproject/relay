# Auth Service
Handles user registration, login, JWT access + refresh tokens, TOTP MFA, OAuth (GitHub/Google), and SAML/OIDC SSO.

**Port:** 8080 (default)
**Health:** `GET /health`

## Running locally
```bash
cd services/auth
go run ./cmd
```
