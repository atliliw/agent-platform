#!/usr/bin/env bash
set -u
SESSION="verify-$(date +%s)"
echo "=== SESSION=$SESSION ==="

echo "=== EXECUTE ==="
BODY=$(printf '{"session_id":"%s","tenant_id":"default","user_id":"verify","message":"Hello, please introduce yourself in one short sentence.","entry_agent":"main-agent"}' "$SESSION")
curl -s --max-time 120 -X POST http://localhost:9000/api/v2/agents/execute \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: default" \
  -d "$BODY"
echo ""

echo "=== MEMORY: session ==="
curl -s --max-time 15 http://localhost:9000/api/v2/memory/session/$SESSION -H "X-Tenant-ID: default"
echo ""

echo "=== MEMORY: all (filter for session) ==="
curl -s --max-time 15 http://localhost:9000/api/v2/memory/all -H "X-Tenant-ID: default" | grep -o "$SESSION" | head -3
echo "(count of session matches above)"

echo "=== CHECKPOINTS IN MONGO ==="
docker exec docker-mongo-1 mongosh --quiet agent_platform --eval "db.agent_checkpoints.find({session_id:'$SESSION'},{step:1,agent_id:1,created_at:1,_id:0}).toArray()"
