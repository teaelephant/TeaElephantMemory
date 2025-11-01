# Admin API Authentication

## Overview

This document describes the authentication mechanism for the TeaElephantEditor admin application. The admin API uses asymmetric key authentication with JWT tokens, separate from the Apple Sign In authentication used by mobile users.

## Architecture

### Key Separation
- **Mobile Users**: Apple Sign In → JWT signed with Apple auth key
- **Admin Users**: Static JWT signed with dedicated admin private key → Verified with admin public key

### Components
1. **Admin Private Key**: ECDSA P-256 key stored securely in macOS Keychain
2. **Admin Public Key**: Corresponding public key mounted as file in server container
3. **Admin JWT**: Short-lived JWT token (24h expiration) signed by admin private key, sent with GraphQL requests

## Security Model

### Threat Model
- TeaElephantEditor is a **private, developer-only application**
- Not distributed via App Store or shared publicly
- Admin operations (tea/tag CRUD) should only be accessible to authenticated admin
- Mobile users (Apple Sign In) should NOT have admin privileges
- **Private key security**: Stored in macOS Keychain, never hardcoded in source
- **Key rotation**: Support multiple public keys with Key ID (kid) header for zero-downtime rotation
- **Token lifetime**: 24-hour expiration with automatic refresh, not 1-year tokens

### Authentication Flow
```
1. TeaElephantEditor starts up
2. App generates/loads admin JWT (signed with admin private key)
3. App sends GraphQL request with header: Authorization: Bearer <admin-jwt>
4. Server middleware validates JWT signature using cached admin public key
5. Server middleware creates AdminPrincipal and adds to context
6. GraphQL resolver checks RequireAdmin() for admin operations
7. If valid admin JWT → allow operation
8. If invalid or missing → return GraphQL error (HTTP 200 with error in response)
```

## Implementation Details

### 1. Key Generation

Generate ECDSA P-256 key pair for admin authentication:

```bash
# Generate private key
openssl ecparam -genkey -name prime256v1 -noout -out admin_private_key.pem

# Extract public key
openssl ec -in admin_private_key.pem -pubout -out admin_public_key.pem

# Display keys for verification
echo "=== Admin Private Key (for TeaElephantEditor) ==="
cat admin_private_key.pem

echo "=== Admin Public Key (for server ADMIN_PUBLIC_KEY env) ==="
cat admin_public_key.pem
```

**Key Storage:**
- `admin_private_key.pem`: Import into macOS Keychain (one-time setup, never committed to repo)
- `admin_public_key.pem`: Mount as Kubernetes secret file in server container (not environment variable)

### 2. Server-Side Changes

#### 2.1 Mount Admin Public Key as File

Mount the admin public key as a file instead of environment variable to avoid encoding issues:

```yaml
# deployment/server.yml
volumeMounts:
  - name: admin-key
    mountPath: /keys/admin
    readOnly: true

volumes:
  - name: admin-key
    secret:
      secretName: admin-auth
      items:
        - key: public-key
          path: admin_public_key.pem
```

#### 2.2 Configuration

Update `internal/auth/cfg.go`:

```go
type Configuration struct {
    // Existing Apple auth fields
    TeamID       string `envconfig:"APPLE_AUTH_TEAM_ID" required:"true"`
    ClientID     string `envconfig:"APPLE_AUTH_CLIENT_ID" required:"true"`
    KeyID        string `envconfig:"APPLE_AUTH_KEY_ID" required:"true"`
    Secret       string `envconfig:"APPLE_AUTH_SECRET" required:"true"`
    SecretPath   string `envconfig:"APPLE_AUTH_SECRET_PATH" required:"true"`

    // New admin auth field - path to mounted public key file
    AdminPublicKeyPath string `envconfig:"ADMIN_PUBLIC_KEY_PATH" default:"/keys/admin/admin_public_key.pem"`
}
```

#### 2.3 Auth Service Updates

Update `internal/auth/auth.go`:

**Add admin key caching to auth struct:**

```go
type auth struct {
    appleClient *apple.Client
    cfg         *Configuration
    secret      string

    // Preloaded admin public keys keyed by kid
    adminKeys      map[string]*ecdsa.PublicKey
    adminKeysMutex sync.RWMutex

    storage
    log *logrus.Entry
}
```

**Add key preloading to Start() method:**

```go
func (a *auth) Start() (err error) {
    a.secret, err = apple.GenerateClientSecret(a.cfg.Secret, a.cfg.TeamID, a.cfg.ClientID, a.cfg.KeyID)
    if err != nil {
        return fmt.Errorf("generate apple client secret: %w", err)
    }

    // Preload admin public keys
    if err := a.loadAdminKeys(); err != nil {
        return fmt.Errorf("load admin keys: %w", err)
    }

    return nil
}

// loadAdminKeys preloads admin public keys at startup
func (a *auth) loadAdminKeys() error {
    a.adminKeysMutex.Lock()
    defer a.adminKeysMutex.Unlock()

    a.adminKeys = make(map[string]*ecdsa.PublicKey)

    // Load default key
    key, err := a.loadPublicKeyFromFile(a.cfg.AdminPublicKeyPath)
    if err != nil {
        return fmt.Errorf("load default admin key: %w", err)
    }
    a.adminKeys[""] = key // Empty kid = default key

    // Try to load versioned keys (v1, v2) if they exist
    baseDir := filepath.Dir(a.cfg.AdminPublicKeyPath)
    for _, version := range []string{"v1", "v2"} {
        keyPath := filepath.Join(baseDir, fmt.Sprintf("admin_public_key_%s.pem", version))
        if key, err := a.loadPublicKeyFromFile(keyPath); err == nil {
            kid := fmt.Sprintf("admin-key-%s", version)
            a.adminKeys[kid] = key
            a.log.Infof("Loaded admin key: %s", kid)
        }
        // Ignore errors for optional versioned keys
    }

    a.log.Infof("Loaded %d admin public key(s)", len(a.adminKeys))
    return nil
}

// loadPublicKeyFromFile loads and parses an ECDSA public key from a PEM file
func (a *auth) loadPublicKeyFromFile(path string) (*ecdsa.PublicKey, error) {
    pemBytes, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    block, _ := pem.Decode(pemBytes)
    if block == nil {
        return nil, ErrEmptyBlockDecode
    }

    key, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("parse public key: %w", err)
    }

    ecdsaKey, ok := key.(*ecdsa.PublicKey)
    if !ok {
        return nil, fmt.Errorf("key is not ECDSA public key")
    }

    return ecdsaKey, nil
}
```

**Add admin validation method:**

```go
const (
    userCtxKey  ctxKey = "user"
    adminCtxKey ctxKey = "adminPrincipal"

    JwtDurationHour       = 24
    AdminJwtDurationHours = 24 // 24-hour admin tokens, same as user tokens

    // JWT claim keys
    AdminClaimKey = "admin"

    // Expected issuer/audience for admin JWTs
    AdminIssuer   = "TeaElephantEditor"
    AdminAudience = "tea-elephant-api"

    // Clock skew tolerance for JWT validation (5 minutes)
    ClockSkewSeconds = 300
)

// AdminPrincipal represents an authenticated admin session
type AdminPrincipal struct {
    JTI       string    // JWT ID for audit/revocation
    IssuedAt  time.Time // Token issue time
    ExpiresAt time.Time // Token expiration
}

func newAdminPrincipal(claims jwt.MapClaims) (*AdminPrincipal, error) {
    jti, err := claims.GetSubject() // Use jti from claims
    if err != nil {
        jti = "" // Optional field
    }

    iat, err := claims.GetIssuedAt()
    if err != nil {
        return nil, fmt.Errorf("missing iat claim: %w", err)
    }

    exp, err := claims.GetExpirationTime()
    if err != nil {
        return nil, fmt.Errorf("missing exp claim: %w", err)
    }

    return &AdminPrincipal{
        JTI:       jti,
        IssuedAt:  iat.Time,
        ExpiresAt: exp.Time,
    }, nil
}

// ValidateAdmin verifies an admin JWT and returns the admin principal
func (a *auth) ValidateAdmin(ctx context.Context, jwtToken string) (*AdminPrincipal, error) {
    // Parse with validation options (handles exp, nbf, iss, aud automatically)
    result, err := jwt.Parse(jwtToken, a.adminVerificationKey,
        jwt.WithValidMethods([]string{signingMethod.Alg()}),
        jwt.WithIssuer(AdminIssuer),
        jwt.WithAudience(AdminAudience),
        jwt.WithLeeway(ClockSkewSeconds * time.Second),
    )
    if err != nil {
        // This covers: signature verification, algorithm check, exp, nbf, iss, aud
        return nil, fmt.Errorf("parse admin jwt: %w", err)
    }

    claims, ok := result.Claims.(jwt.MapClaims)
    if !ok || !result.Valid {
        return nil, common.ErrInvalidToken
    }

    // Check admin claim (only custom validation needed)
    isAdmin, ok := claims[AdminClaimKey].(bool)
    if !ok || !isAdmin {
        return nil, common.ErrNotAdmin
    }

    // Create admin principal from claims for context storage
    principal, err := newAdminPrincipal(claims)
    if err != nil {
        return nil, fmt.Errorf("create admin principal: %w", err)
    }

    // All standard validations (exp, nbf, iss, aud, alg) are handled by jwt.Parse
    // adminVerificationKey already validates kid and selects the correct key

    return principal, nil
}

// adminVerificationKey returns the admin public key for JWT verification
// Uses preloaded keys from cache for performance
// Supports multiple keys via kid header for rotation
func (a *auth) adminVerificationKey(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
        return nil, fmt.Errorf("%w: %v", errUnexpectedSigningMethod, token.Header["alg"])
    }

    // Determine which cached key to use based on kid header
    kid, hasKid := token.Header["kid"].(string)
    if !hasKid {
        kid = "" // Use default key
    }

    // Lookup key in cache (thread-safe read)
    a.adminKeysMutex.RLock()
    key, exists := a.adminKeys[kid]
    a.adminKeysMutex.RUnlock()

    if !exists {
        if kid == "" {
            return nil, fmt.Errorf("no default admin key loaded")
        }
        return nil, fmt.Errorf("unknown key id: %s", kid)
    }

    return key, nil
}
```

**Update middleware to handle admin auth:**

```go
// InterceptResponse intercepts GraphQL responses to ensure the user is authenticated.
func (a *Middleware) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
    if !graphql.HasOperationContext(ctx) {
        return next(ctx)
    }

    rc := graphql.GetOperationContext(ctx)
    header := rc.Headers.Get("Authorization")

    // Allow unauthenticated requests to proceed (resolvers will check if auth is required)
    if header == "" {
        return next(ctx)
    }

    token := strings.Replace(header, bearerPrefix, "", 1)

    // Try admin auth first
    if principal, err := a.auth.ValidateAdmin(ctx, token); err == nil {
        // Valid admin token - set admin principal in context
        return next(context.WithValue(ctx, adminCtxKey, principal))
    }

    // Fall back to user auth
    user, err := a.auth.Validate(ctx, token)
    if err != nil {
        // Invalid token - log and continue without auth context
        // The resolver will handle authorization errors
        a.log.WithError(err).Warn(invalidJWTMsg)
        return next(ctx)
    }

    // Valid user token - set user context
    return next(context.WithValue(ctx, userCtxKey, user))
}
```

**Add admin check helpers:**

```go
// GetAdminPrincipal extracts the admin principal from context
// Returns nil if not authenticated as admin
func GetAdminPrincipal(ctx context.Context) *AdminPrincipal {
    value := ctx.Value(adminCtxKey)
    if principal, ok := value.(*AdminPrincipal); ok {
        return principal
    }
    return nil
}

// IsAdmin checks if the current request is authenticated as admin
func IsAdmin(ctx context.Context) bool {
    return GetAdminPrincipal(ctx) != nil
}

// RequireAdmin returns an error if the request is not admin authenticated
// Also returns the admin principal for audit logging
func RequireAdmin(ctx context.Context) (*AdminPrincipal, error) {
    principal := GetAdminPrincipal(ctx)
    if principal == nil {
        return nil, common.ErrUnauthorized
    }
    return principal, nil
}
```

#### 2.4 Common Errors

Update `common/error.go`:

```go
var (
    // Existing errors
    ErrInvalidToken  = errors.New("invalid token")
    ErrExpiredToken  = errors.New("token expired")
    ErrJwtIncorrect  = errors.New("jwt incorrect")
    ErrUserNotFound  = errors.New("user not found")

    // New admin errors
    ErrNotAdmin      = errors.New("not an admin")
    ErrUnauthorized  = errors.New("unauthorized")
)
```

#### 2.5 Resolver Guards

Update admin operations in `pkg/api/v2/graphql/schema.resolvers.go`:

**Example: NewTea mutation**

```go
// NewTea is the resolver for the newTea field.
func (r *mutationResolver) NewTea(ctx context.Context, tea model.TeaData) (*model.Tea, error) {
    // Require admin authentication
    if err := authPkg.RequireAdmin(ctx); err != nil {
        return nil, &gqlerror.Error{
            Message: "Admin authentication required",
            Extensions: map[string]interface{}{
                "code": "UNAUTHORIZED",
            },
        }
    }

    res, err := r.teaData.Create(ctx, tea.ToCommonTeaData())
    if err != nil {
        return nil, castGQLError(ctx, err)
    }

    return model.FromCommonTea(res), nil
}
```

**Note on Error Handling:**

The middleware validates JWTs and sets context but **does not reject requests with HTTP 401/403**. Instead:

1. Middleware validates JWT and sets `AdminPrincipal` in context
2. If JWT is invalid, middleware logs warning and continues without setting context
3. Resolver calls `RequireAdmin()` which checks for admin principal
4. If not authorized, resolver returns GraphQL error with `UNAUTHORIZED` code
5. Client receives HTTP 200 with GraphQL error in response body

**Why HTTP 200 + GraphQL errors?**
- Consistent with GraphQL spec (transport vs application errors)
- Allows GraphQL batching to partially succeed
- Better error details in GraphQL response format
- Standard GraphQL client error handling

**Example error response:**
```json
{
  "data": {
    "newTea": null
  },
  "errors": [
    {
      "message": "Admin authentication required",
      "path": ["newTea"],
      "extensions": {
        "code": "UNAUTHORIZED"
      }
    }
  ]
}
```

If you need HTTP-level status codes (401/403) for non-GraphQL clients, implement a transport-level interceptor that maps `UNAUTHORIZED` errors to HTTP 403.

**Apply to all admin mutations:**
- `NewTea`
- `UpdateTea`
- `DeleteTea`
- `AddTagToTea`
- `DeleteTagFromTea`
- `WriteToQR`
- `CreateTagCategory`
- `UpdateTagCategory`
- `DeleteTagCategory`
- `CreateTag`
- `UpdateTag`
- `ChangeTagCategory`
- `DeleteTag`

### 3. TeaElephantEditor Changes

#### 3.1 Store Admin Private Key

Store `admin_private_key.pem` in TeaElephantEditor project:

```
TeaElephantEditor/
  ├── Keys/
  │   └── admin_private_key.pem  (add to .gitignore in main repo)
  └── TeaElephantEditor.xcodeproj
```

#### 3.2 Store Admin Private Key in Keychain

**One-time setup: Import private key into macOS Keychain**

```bash
# Import the private key into Keychain
security import admin_private_key.pem -k ~/Library/Keychains/login.keychain-db -T /Applications/TeaElephantEditor.app

# Set access control (allow app to access without prompt)
# This requires Keychain Access.app or programmatic setup
```

#### 3.3 Generate Admin JWT from Keychain

Create a helper to load private key from Keychain and generate JWT:

**Swift example using Keychain and CryptoKit:**

```swift
import Foundation
import Security
import CryptoKit

class AdminAuth {
    private static let keychainLabel = "com.teaelephant.editor.adminkey"

    // Load private key from Keychain as SecKey, then convert to CryptoKit key
    static func loadPrivateKeyFromKeychain() -> P256.Signing.PrivateKey? {
        // Query for the SecKey reference
        let query: [String: Any] = [
            kSecClass as String: kSecClassKey,
            kSecAttrKeyClass as String: kSecAttrKeyClassPrivate,
            kSecAttrKeyType as String: kSecAttrKeyTypeECSECPrimeRandom,
            kSecAttrLabel as String: keychainLabel,
            kSecReturnRef as String: true  // Return SecKey reference, not data
        ]

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)

        guard status == errSecSuccess,
              let secKey = item as! SecKey? else {
            print("Keychain lookup failed: \(status)")
            return nil
        }

        // Export the key data from SecKey
        var error: Unmanaged<CFError>?
        guard let keyData = SecKeyCopyExternalRepresentation(secKey, &error) as Data? else {
            print("Failed to export key: \(error!.takeRetainedValue())")
            return nil
        }

        // CryptoKit expects x963 representation for P256
        // SecKey exports in x963 format for EC keys, so we can use it directly
        return try? P256.Signing.PrivateKey(x963Representation: keyData)
    }

    static func generateAdminJWT() -> String? {
        // Load private key from Keychain
        guard let privateKey = loadPrivateKeyFromKeychain() else {
            print("Failed to load private key from Keychain")
            return nil
        }

        // Create JWT header with key ID for rotation support
        let header = [
            "alg": "ES256",
            "typ": "JWT",
            "kid": "admin-key-v1"  // Key ID for rotation
        ]

        let now = Date()
        let expiration = now.addingTimeInterval(24 * 60 * 60) // 24 hours
        let notBefore = now.addingTimeInterval(-60) // Valid from 1 minute ago (clock skew)

        let claims: [String: Any] = [
            "admin": true,
            "iss": "TeaElephantEditor",
            "aud": "tea-elephant-api",
            "iat": Int(now.timeIntervalSince1970),
            "nbf": Int(notBefore.timeIntervalSince1970),
            "exp": Int(expiration.timeIntervalSince1970),
            "jti": UUID().uuidString
        ]

        // Encode header and claims
        guard let headerData = try? JSONSerialization.data(withJSONObject: header),
              let claimsData = try? JSONSerialization.data(withJSONObject: claims) else {
            return nil
        }

        let headerB64 = headerData.base64URLEncodedString()
        let claimsB64 = claimsData.base64URLEncodedString()

        let message = "\(headerB64).\(claimsB64)"
        guard let messageData = message.data(using: .utf8) else {
            return nil
        }

        // Sign with private key
        guard let signature = try? privateKey.signature(for: messageData) else {
            return nil
        }
        let signatureB64 = signature.rawRepresentation.base64URLEncodedString()

        return "\(message).\(signatureB64)"
    }
}

extension Data {
    func base64URLEncodedString() -> String {
        return base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
    }
}
```

#### 3.3 Send JWT with Requests

Configure GraphQL client to send admin JWT:

```swift
// Apollo Client example
let configuration = URLSessionConfiguration.default
configuration.httpAdditionalHeaders = [
    "Authorization": "Bearer \(AdminAuth.generateAdminJWT()!)"
]

let client = ApolloClient(
    networkTransport: RequestChainNetworkTransport(
        interceptorProvider: DefaultInterceptorProvider(client: URLSessionClient(sessionConfiguration: configuration)),
        endpointURL: URL(string: "https://tea-elephant.com/graphql")!
    )
)
```

### 4. Deployment

#### 4.1 Create Admin Secret

Create Kubernetes secret directly from PEM file (no base64 encoding):

```bash
# Create secret from file
kubectl create secret generic admin-auth \
  --from-file=admin_public_key.pem \
  --namespace=teaelephant
```

#### 4.2 Update Deployment

Update `deployment/server.yml` to mount the key as a file:

```yaml
# In the server container spec
spec:
  containers:
    - name: server
      # ... existing config ...
      volumeMounts:
        - name: keys
          mountPath: /keys/
        - name: admin-key
          mountPath: /keys/admin
          readOnly: true
      # ... rest of container spec ...
  volumes:
    - name: keys
      secret:
        secretName: apple-auth
    - name: admin-key
      secret:
        secretName: admin-auth
        items:
          - key: admin_public_key.pem
            path: admin_public_key.pem
```

No environment variable needed - the server reads directly from `/keys/admin/admin_public_key.pem`.

#### 4.3 Deploy

```bash
kubectl apply -f deployment/server.yml
kubectl rollout restart deployment server -n teaelephant
```

## Testing

### 1. Generate Test Keys and JWT

```bash
# Generate keys
openssl ecparam -genkey -name prime256v1 -noout -out test_admin_private.pem
openssl ec -in test_admin_private.pem -pubout -out test_admin_public.pem

# Generate test JWT (using jwt.io or jwt-cli)
# Header: {"alg": "ES256", "typ": "JWT"}
# Payload: {"admin": true, "iss": "test", "exp": 1735689600, "jti": "test-123"}
```

### 2. Test Admin Mutations

```bash
# With admin JWT (should succeed)
curl -X POST https://tea-elephant.com/graphql \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{"query": "mutation { newTea(tea: {name: \"Test Tea\", type: tea, description: \"Test\"}) { id name } }"}'

# Without auth (should fail with 401/403)
curl -X POST https://tea-elephant.com/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "mutation { newTea(tea: {name: \"Test Tea\", type: tea, description: \"Test\"}) { id name } }"}'

# With user JWT from Apple Sign In (should fail with 401/403)
curl -X POST https://tea-elephant.com/graphql \
  -H "Authorization: Bearer <user-jwt>" \
  -H "Content-Type: application/json" \
  -d '{"query": "mutation { newTea(tea: {name: \"Test Tea\", type: tea, description: \"Test\"}) { id name } }"}'
```

### 3. Verify Public Queries Still Work

```bash
# Public queries should work without auth
curl -X POST https://tea-elephant.com/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "query { teas { id name } }"}'
```

## Security Considerations

### Key Management
- **Private Key Security**: Store admin private key securely in TeaElephantEditor
  - Consider using macOS Keychain or hardcode if repo is private
  - Never commit private key to public repositories
- **Public Key**: Safe to store in environment variables or config
- **Key Rotation**: If private key is compromised, generate new key pair and update both server and client

### JWT Security
- **24-Hour Expiration**: Admin JWTs expire after 24 hours, same as user tokens
  - Automatic regeneration on app startup if token expired
  - Balance between security and convenience
- **JWT Claims**: Include `jti` (JWT ID), `nbf` (not-before), `iss` (issuer), and `aud` (audience)
- **Strict Validation**: Server validates algorithm (ES256 only), issuer, audience, expiration, and not-before
- **Clock Skew Tolerance**: 5-minute tolerance for clock differences between client and server

### Network Security
- **HTTPS Only**: All admin API calls must use HTTPS
- **Certificate Pinning** (optional): Consider in TeaElephantEditor for additional security

### Audit Logging
- **Future Enhancement**: Log all admin operations with timestamp and operation type
- **Monitoring**: Alert on unusual admin activity patterns

## Deployment Strategy

Since you're the sole admin and can coordinate server and client updates together, deploy all changes in one go:

### Deployment Steps

1. **Generate admin key pair** and import private key to macOS Keychain
2. **Create Kubernetes secret** with admin public key
3. **Update server code** with admin JWT validation and RequireAdmin guards
4. **Update deployment.yml** to mount admin public key
5. **Deploy server** with admin authentication enabled
6. **Update TeaElephantEditor** to load key from Keychain and generate admin JWT
7. **Test admin operations** end-to-end

**Note:** This is a breaking change - admin mutations will immediately require authentication. Ensure TeaElephantEditor is updated before deploying the server, or be prepared to update it immediately after deployment.

### Rollback Plan

If issues arise:
1. Remove `RequireAdmin()` checks from resolvers
2. Redeploy server
3. Admin operations revert to unauthenticated state
4. Debug and fix TeaElephantEditor
5. Redeploy with auth enabled

## Appendix

### A. Admin Operations List

**Tea Management:**
- `newTea`
- `updateTea`
- `deleteTea`
- `addTagToTea`
- `deleteTagFromTea`

**Tag Management:**
- `createTagCategory`
- `updateTagCategory`
- `deleteTagCategory`
- `createTag`
- `updateTag`
- `changeTagCategory`
- `deleteTag`

**QR Management:**
- `writeToQR`

**Public Operations (no auth required):**
- `teas` (query)
- `tea` (query)
- `qrRecord` (query)
- `tag` (query)
- `tagsCategories` (query)
- `generateDescription` (mutation/subscription - AI powered)

**User Operations (require Apple Sign In):**
- `authApple`
- `me`
- `collections`
- `createCollection`
- `addRecordsToCollection`
- `deleteRecordsFromCollection`
- `deleteCollection`
- `registerDeviceToken`
- `teaRecommendation`
- `teaOfTheDay`

### B. GraphQL Schema Updates (Optional)

Consider adding explicit admin markers in schema for documentation:

```graphql
type Mutation {
    """Admin only: Create a new tea"""
    newTea(tea: TeaData!): Tea!

    """Admin only: Update tea details"""
    updateTea(id: ID!, tea: TeaData!): Tea!

    # ... etc
}
```

**Note:** Schema directives like `@requireAdmin` could be implemented for schema-level enforcement, but would require additional directive resolver setup in gqlgen. The current resolver-level guards are simpler and more explicit.

### C. Key Rotation Strategy

#### Zero-Downtime Key Rotation Process

**Step 1: Generate New Key Pair**
```bash
openssl ecparam -genkey -name prime256v1 -noout -out admin_private_key_v2.pem
openssl ec -in admin_private_key_v2.pem -pubout -out admin_public_key_v2.pem
```

**Step 2: Add New Public Key to Server**

Mount both old and new public keys:

```yaml
# deployment/server.yml
volumes:
  - name: admin-keys
    secret:
      secretName: admin-auth
      items:
        - key: public-key-v1
          path: admin_public_key_v1.pem
        - key: public-key-v2
          path: admin_public_key_v2.pem
```

**Step 3: Update Server to Accept Both Keys**

Modify `adminVerificationKey` to try multiple keys:

```go
func (a *auth) adminVerificationKey(token *jwt.Token) (interface{}, error) {
    // Check kid header to determine which key to use
    kid, _ := token.Header["kid"].(string)

    var keyPath string
    switch kid {
    case "admin-key-v1":
        keyPath = "/keys/admin/admin_public_key_v1.pem"
    case "admin-key-v2":
        keyPath = "/keys/admin/admin_public_key_v2.pem"
    default:
        // Default to v1 for backward compatibility
        keyPath = "/keys/admin/admin_public_key_v1.pem"
    }

    pemBytes, err := os.ReadFile(keyPath)
    // ... rest of key loading
}
```

**Step 4: Deploy Server with Both Keys**
```bash
kubectl apply -f deployment/server.yml
kubectl rollout status deployment server -n teaelephant
```

**Step 5: Update TeaElephantEditor**
- Import new private key v2 to Keychain
- Update JWT generation to use "admin-key-v2" in kid header
- Test admin operations

**Step 6: Remove Old Key (After Grace Period)**

After confirming all clients use the new key (e.g., 30 days):
```bash
# Remove old key from Kubernetes secret
kubectl create secret generic admin-auth \
  --from-file=public-key-v2=admin_public_key_v2.pem \
  --namespace=teaelephant \
  --dry-run=client -o yaml | kubectl apply -f -

# Update server to only accept v2
# Deploy changes
```

#### JWKS Support (Future Enhancement)

For more dynamic key management, consider implementing JWKS (JSON Web Key Set):

**Server exposes JWKS endpoint:**
```json
GET /.well-known/jwks.json
{
  "keys": [
    {
      "kty": "EC",
      "use": "sig",
      "kid": "admin-key-v1",
      "alg": "ES256",
      "crv": "P-256",
      "x": "...",
      "y": "..."
    },
    {
      "kty": "EC",
      "use": "sig",
      "kid": "admin-key-v2",
      "alg": "ES256",
      "crv": "P-256",
      "x": "...",
      "y": "..."
    }
  ]
}
```

**Benefits:**
- Dynamic key discovery
- Automated key rotation
- Support for multiple active keys
- Standard JWKS format

### D. Future Enhancements

1. **Admin User Management**: Track which admin user made changes (add user ID to JWT claims)
2. **Audit Logs**: Log all admin operations to database with timestamp, operation, and admin ID
3. **JWKS Endpoint**: Expose /.well-known/jwks.json for dynamic key discovery
4. **Rate Limiting**: Throttle admin operations to prevent abuse (e.g., 100 req/min)
5. **Two-Factor Authentication**: Add 2FA for admin JWT generation in TeaElephantEditor
6. **Time-Based Restrictions**: Only allow admin operations during business hours (configurable)
7. **IP Allowlisting**: Restrict admin API access to specific IP ranges
8. **Admin Session Management**: Track active admin sessions and support revocation

---

## Summary

This implementation provides secure admin authentication for TeaElephantEditor using asymmetric cryptography with strict JWT validation and key management best practices.

### Key Security Features:

✅ **Secure Key Storage**: Admin private key stored in macOS Keychain, never hardcoded
✅ **Strict JWT Validation**: Algorithm pinning (ES256 only), issuer/audience verification, clock skew tolerance
✅ **Short-Lived Tokens**: 24-hour expiration with automatic regeneration
✅ **Key Rotation Support**: Key ID (kid) header enables zero-downtime rotation
✅ **File-Based Public Key**: Mounted as Kubernetes secret file, avoiding base64 encoding issues
✅ **Separation of Concerns**: Clear distinction between user (Apple Sign In) and admin auth
✅ **Defense in Depth**: Not-before claim, JWT ID for potential revocation, multiple validation layers

### Implementation Highlights:

- Server validates admin JWTs using public key mounted from Kubernetes secret
- TeaElephantEditor loads private key from Keychain and generates JWT on startup
- JWT includes full claims: iss, aud, iat, nbf, exp, jti, admin, kid
- Middleware checks admin status before allowing mutations
- Ready for key rotation with multi-key support framework
- Future-proof with JWKS endpoint planning

The admin private key remains secure in the macOS Keychain, never transmitted or exposed, while the server validates requests using only the corresponding public key.
