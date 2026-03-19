# Auth Service API

**Port:** 8001 (HTTP) / 9001 (gRPC)

The auth service handles user registration, login, token management, OAuth, password reset, email verification, session management, and MFA.

## Authentication

Most endpoints are public. Endpoints requiring authentication use a JWT Bearer token:

```
Authorization: Bearer <access_token>
```

Middleware: `middleware.Auth(jwtSecret)` — validates JWT signature and expiration; extracts claims via `middleware.GetClaims(r.Context())`.

## REST Endpoints

### Registration & Login

#### `POST /api/v1/auth/register`

Register a new user account.

- **Auth:** None
- **Request body:**
  ```json
  {
    "email": "string",
    "password": "string",
    "display_name": "string"
  }
  ```
- **Response `200`:**
  ```json
  {
    "user_id": "string",
    "message": "string"
  }
  ```

#### `POST /api/v1/auth/login`

Authenticate a user and issue tokens.

- **Auth:** None
- **Request body:**
  ```json
  {
    "email": "string",
    "password": "string",
    "totp_code": "string"
  }
  ```
- **Response `200`:**
  ```json
  {
    "access_token": "string",
    "refresh_token": "string",
    "expires_in": 3600
  }
  ```

#### `POST /api/v1/auth/logout`

Invalidate a refresh token.

- **Auth:** None
- **Request body:**
  ```json
  { "refresh_token": "string" }
  ```
- **Response `200`:**
  ```json
  { "ok": true }
  ```

#### `POST /api/v1/auth/refresh`

Rotate access and refresh tokens.

- **Auth:** None
- **Request body:**
  ```json
  { "refresh_token": "string" }
  ```
- **Response `200`:**
  ```json
  {
    "access_token": "string",
    "refresh_token": "string",
    "expires_in": 3600
  }
  ```

### OAuth

#### `GET /api/v1/auth/oauth/{provider}`

Redirect user to OAuth provider.

- **Auth:** None
- **Path params:** `provider` — e.g. `google`, `github`
- **Query params:** `state` (string)
- **Response:** HTTP redirect to provider authorization URL

#### `POST /api/v1/auth/oauth/{provider}/callback`

Handle OAuth provider callback and issue tokens.

- **Auth:** None
- **Path params:** `provider`
- **Request body:**
  ```json
  {
    "code": "string",
    "state": "string"
  }
  ```
- **Response `200`:**
  ```json
  {
    "access_token": "string",
    "refresh_token": "string",
    "expires_in": 3600
  }
  ```

### Password Reset

#### `POST /api/v1/auth/password/reset-request`

Send password reset email.

- **Auth:** None
- **Request body:**
  ```json
  { "email": "string" }
  ```
- **Response `200`:**
  ```json
  { "message": "string" }
  ```

#### `POST /api/v1/auth/password/reset`

Apply a password reset token and set a new password.

- **Auth:** None
- **Request body:**
  ```json
  {
    "token": "string",
    "new_password": "string"
  }
  ```
- **Response `200`:**
  ```json
  { "ok": true }
  ```

### Email Verification

#### `POST /api/v1/auth/verify-email`

Verify email with a token sent via email.

- **Auth:** None
- **Query params:** `token` (string)
- **Response `200`:**
  ```json
  { "ok": true }
  ```

### Sessions

#### `GET /api/v1/auth/sessions`

List active sessions for the authenticated user.

- **Auth:** JWT required
- **Response `200`:**
  ```json
  {
    "sessions": [
      {
        "id": "string",
        "device_name": "string",
        "ip": "string",
        "last_seen_at": "2024-01-01T00:00:00Z",
        "created_at": "2024-01-01T00:00:00Z"
      }
    ]
  }
  ```

#### `DELETE /api/v1/auth/sessions/{id}`

Revoke a specific session.

- **Auth:** JWT required
- **Path params:** `id` — session ID
- **Response `200`:**
  ```json
  { "ok": true }
  ```

### MFA

#### `POST /api/v1/auth/mfa/setup`

Initiate TOTP MFA setup; returns secret and QR code URL.

- **Auth:** JWT required
- **Response `200`:**
  ```json
  {
    "totp_secret": "string",
    "qr_code_url": "string"
  }
  ```

#### `POST /api/v1/auth/mfa/verify`

Confirm MFA setup with a TOTP code; returns backup codes.

- **Auth:** JWT required
- **Request body:**
  ```json
  {
    "totp_secret": "string",
    "totp_code": "string"
  }
  ```
- **Response `200`:**
  ```json
  {
    "backup_codes": ["string"],
    "ok": true
  }
  ```

## gRPC Service

**Proto:** `proto/auth/v1/auth.proto`

```protobuf
service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
}
```

### Messages

#### `LoginRequest` / `LoginResponse`

```protobuf
message LoginRequest {
  string email = 1;
  string password = 2;
  string device = 3;
}

message LoginResponse {
  string access_token = 1;
  string refresh_token = 2;
  google.protobuf.Timestamp expires_at = 3;
  string user_id = 4;
}
```

#### `RefreshTokenRequest` / `RefreshTokenResponse`

```protobuf
message RefreshTokenRequest {
  string refresh_token = 1;
}

message RefreshTokenResponse {
  string access_token = 1;
  google.protobuf.Timestamp expires_at = 2;
}
```

#### `ValidateTokenRequest` / `ValidateTokenResponse`

```protobuf
message ValidateTokenRequest {
  string access_token = 1;
}

message ValidateTokenResponse {
  bool valid = 1;
  string user_id = 2;
  string session_id = 3;
}
```

## Error Responses

All errors follow the standard format:

```json
{
  "error": "ERROR_CODE",
  "message": "human-readable description"
}
```

Common codes: `UNAUTHORIZED` (401), `NOT_FOUND` (404), `INVALID_ARGUMENT` (400), `INTERNAL` (500).
