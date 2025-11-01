Admin Keys Setup

This guide shows how to generate admin JWT keys, install the public key into the server deployment, and configure the client to use the private key.

Prerequisites
- openssl installed
- kubectl access to your cluster/namespace (optional for automated apply)

1) Generate keys
Option A: Use the helper script (recommended)
- scripts/generate_admin_keys.sh
- scripts/generate_admin_keys.sh --apply --namespace teaelephant
- Outputs:
  - secrets/admin_private_key.pem (KEEP SECRET; client side)
  - secrets/admin_public_key.pem (install to server)
- With rotation key id (kid): scripts/generate_admin_keys.sh --kid v1

Option B: Manual commands
- openssl ecparam -genkey -name prime256v1 -noout -out secrets/admin_private_key.pem
- openssl ec -in secrets/admin_private_key.pem -pubout -out secrets/admin_public_key.pem

2) Install public key to the server
The deployment expects a secret named admin-auth with a key file named admin_public_key.pem and mounts it at /keys/admin/admin_public_key.pem. The server reads the file via ADMIN_PUBLIC_KEY_PATH.

Apply using the script (Option A above) or apply the example manifest:
- kubectl apply -f deployment/admin-auth.example.yaml

Verify
- kubectl -n teaelephant get secret admin-auth
- Check server deployment mounts:
  - env: ADMIN_PUBLIC_KEY_PATH=/keys/admin/admin_public_key.pem
  - volumeMounts: name: admin-key -> /keys/admin (readOnly)

3) Configure the client (TeaElephantEditor)
- Do NOT commit admin_private_key.pem.
- Prefer storing the private key in macOS Keychain or Secure Enclave.
- Example (CryptoKit):
  - let privateKey = try P256.Signing.PrivateKey(pemRepresentation: privateKeyPEM)
  - let header = ["alg": "ES256", "kid": "v1"] // optional kid if you rotate keys
  - Sign claims with 24h expiry, iss="TeaElephantEditor", aud="tea-elephant-api", admin=true

4) Rotation
- Generate a new key pair using the script with a new --kid (e.g., v2).
- Update the Kubernetes secret with the new public key (script --apply).
- Update client to send tokens with kid=v2 and sign with the new private key.
- After all clients are updated, remove the old key from the secret when ready.

Troubleshooting
- If the server logs show UNAUTHENTICATED for admin operations, ensure:
  - The secret exists and is mounted
  - ADMIN_PUBLIC_KEY_PATH points to /keys/admin/admin_public_key.pem
  - JWT header alg is ES256; claims include admin=true, iss/aud match
