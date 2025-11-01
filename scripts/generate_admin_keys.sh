#!/usr/bin/env bash
set -euo pipefail

# TeaElephantMemory admin key generator and K8s secret helper
#
# This script generates an ECDSA P-256 key pair for admin authentication,
# writes them to ./secrets/admin_private_key.pem and ./secrets/admin_public_key.pem,
# and optionally creates/updates the Kubernetes secret `admin-auth` used by the server
# deployment to mount the public key at /keys/admin/admin_public_key.pem.
#
# Requirements:
# - openssl
# - kubectl (optional, only if you pass --apply)
#
# Usage examples:
#   scripts/generate_admin_keys.sh
#   scripts/generate_admin_keys.sh --kid v1
#   scripts/generate_admin_keys.sh --apply --namespace teaelephant --kid v1
#
# Notes:
# - The server reads the admin public key from ADMIN_PUBLIC_KEY_PATH which defaults to
#   /keys/admin/admin_public_key.pem and is configured in deployment/server.yml.
# - The Kubernetes secret name is `admin-auth` and the key item must be named
#   `admin_public_key.pem` to match the deployment volume items.
# - The private key is for the client (TeaElephantEditor). DO NOT commit it.

NS="teaelephant"
KID=""
APPLY=false
OUT_DIR="secrets"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace|-n)
      NS="$2"; shift 2;;
    --kid)
      KID="$2"; shift 2;;
    --apply)
      APPLY=true; shift 1;;
    --out)
      OUT_DIR="$2"; shift 2;;
    --help|-h)
      echo "Usage: $0 [--namespace <ns>] [--kid <id>] [--apply] [--out <dir>]"; exit 0;;
    *)
      echo "Unknown arg: $1" >&2; exit 1;;
  esac
done

mkdir -p "$OUT_DIR"
PRIV="$OUT_DIR/admin_private_key.pem"
PUB="$OUT_DIR/admin_public_key.pem"

if [[ -f "$PRIV" || -f "$PUB" ]]; then
  echo "Warning: target files already exist. Will not overwrite." >&2
  echo "  $PRIV" >&2
  echo "  $PUB" >&2
  echo "Move/backup these files or use --out to a new dir, then re-run." >&2
  exit 2
fi

# Generate P-256 key pair
openssl ecparam -genkey -name prime256v1 -noout -out "$PRIV"
openssl ec -in "$PRIV" -pubout -out "$PUB"
chmod 600 "$PRIV"

echo "Generated admin key pair:"
echo "  Private: $PRIV"
echo "  Public : $PUB"

if $APPLY; then
  if ! command -v kubectl >/dev/null 2>&1; then
    echo "kubectl not found in PATH; cannot apply secret." >&2
    exit 3
  fi
  # Create or update the secret admin-auth with the expected key name
  # We name the key 'admin_public_key.pem' to match deployment/server.yml items.key
  echo "Applying Kubernetes secret 'admin-auth' in namespace '$NS'..."
  kubectl create secret generic admin-auth \
    --namespace "$NS" \
    --from-file=admin_public_key.pem="$PUB" \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "Done. Verify the mount in the server pod after deploy/restart."
fi

if [[ -n "$KID" ]]; then
  echo
  echo "Suggested JWT header for client (kid support):"
  echo "  { \"alg\": \"ES256\", \"kid\": \"$KID\" }"
fi

echo
cat <<EOF
Next steps:
1) Client (TeaElephantEditor):
   - Import the private key into macOS Keychain or store securely on disk.
   - Use CryptoKit or SecKey APIs to sign ES256 JWTs.
2) Server:
   - Ensure deployment mounts the secret 'admin-auth' and sets ADMIN_PUBLIC_KEY_PATH=/keys/admin/admin_public_key.pem
   - Current repo's deployment/server.yml already matches this.
3) Rotation:
   - Generate a new key pair, update the secret, distribute new kid/token to client, then remove old key when safe.
EOF
