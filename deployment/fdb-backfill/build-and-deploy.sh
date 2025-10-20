#!/bin/bash
# Build and deploy FDB backfill tool

set -e

ROOT_DIR="$(git rev-parse --show-toplevel)"
cd "$ROOT_DIR"

echo "========================================="
echo "FDB → PostgreSQL Backfill Deployment"
echo "========================================="
echo ""

# Step 1: Check if FDB dependencies are restored
echo "Step 1: Checking FDB dependencies..."
if [ ! -f "pkg/fdbclient/client.go" ]; then
    echo "⚠️  FDB dependencies not found. Restoring from git..."
    cd cmd/backfill
    ./restore_fdb_deps.sh
    cd "$ROOT_DIR"
else
    echo "✓ FDB dependencies already restored"
fi
echo ""

# Step 2: Build Docker image
echo "Step 2: Building Docker image with FDB support..."
IMAGE_TAG="${1:-ghcr.io/teaelephant/teaelephantmemory:fdb-backfill}"
echo "Building: $IMAGE_TAG"

docker build -f Dockerfile.fdb -t "$IMAGE_TAG" .
echo "✓ Docker image built"
echo ""

# Step 3: Push to registry (optional)
read -p "Push image to registry? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Pushing $IMAGE_TAG..."
    docker push "$IMAGE_TAG"
    echo "✓ Image pushed"
else
    echo "Skipping image push"
fi
echo ""

# Step 4: Update FDB cluster config
echo "Step 4: Reviewing FDB cluster configuration..."
echo "Current config/fdb.cluster:"
cat config/fdb.cluster
echo ""
read -p "Is this FDB cluster configuration correct? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Please update config/fdb.cluster and deployment/fdb-backfill/server.yml"
    exit 1
fi
echo ""

# Step 5: Deploy to Kubernetes
read -p "Deploy to Kubernetes? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deploying to Kubernetes..."
    kubectl apply -f deployment/fdb-backfill/server.yml
    echo "✓ Deployment created"
    echo ""

    echo "To watch backfill progress:"
    echo "  kubectl logs -n teaelephant -f deployment/server-fdb-backfill"
    echo ""
    echo "To check pod status:"
    echo "  kubectl get pods -n teaelephant -l app=server-fdb-backfill"
    echo ""
    echo "After successful completion, clean up with:"
    echo "  kubectl delete -f deployment/fdb-backfill/server.yml"
else
    echo "Skipping Kubernetes deployment"
    echo ""
    echo "To deploy manually:"
    echo "  kubectl apply -f deployment/fdb-backfill/server.yml"
fi
echo ""

echo "========================================="
echo "Build process complete!"
echo "========================================="
