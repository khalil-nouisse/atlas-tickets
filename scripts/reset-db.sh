#!/bin/bash
set -e

echo "Resetting AtlasTickets Database..."

# PostgreSQL
echo "Resetting PostgreSQL..."
kubectl exec -i postgres-0 -n atlastickets -- psql -U atlas -d atlastickets <<EOF
TRUNCATE TABLE bookings;
UPDATE ticket_inventory SET sold_seats = 0;
SELECT match_id, category, total_seats, sold_seats FROM ticket_inventory;
EOF

# MongoDB
echo "Resetting MongoDB..."
kubectl exec -i mongo-0 -n atlastickets -- mongosh tickets_read_db --quiet --eval "
db.bookings.deleteMany({});
print('Bookings deleted:', db.bookings.find().count());
"

# Redis
echo "Flushing Redis..."
kubectl exec -i $(kubectl get pod -n atlastickets -l app=redis -o jsonpath='{.items[0].metadata.name}') -n atlastickets -- redis-cli FLUSHALL

# Restart query service to repopulate Redis
echo "Restarting query service..."
kubectl rollout restart deployment query-service -n atlastickets

echo "Database reset complete!"