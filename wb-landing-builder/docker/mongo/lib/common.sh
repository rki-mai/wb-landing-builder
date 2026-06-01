#!/bin/bash

INIT_FLAG="/data/db/.initialized"
MONGOD_PID=""

cleanup() {
  if [[ -n "${MONGOD_PID:-}" ]] && kill -0 "$MONGOD_PID" 2>/dev/null; then
    kill -TERM "$MONGOD_PID" 2>/dev/null || true
    wait "$MONGOD_PID" 2>/dev/null || true
  fi
}

start_mongod() {
  mongod --replSet rs0 --bind_ip_all &
  MONGOD_PID=$!
}

wait_for_mongo() {
  until /usr/local/bin/mongo-ping.sh; do
    sleep 1
  done
}

initialize_mongo() {
  if [[ -f "$INIT_FLAG" ]]; then
    echo "MongoDB already initialized"
    return 0
  fi

  echo "Initializing replica set..."
  mongosh --quiet --eval "
    try {
      const status = rs.status();
      print('Replica set already initialized: ' + status.set);
    } catch (e) {
      rs.initiate({ _id: 'rs0', members: [{ _id: 0, host: 'mongo:27017' }] });
      print('Replica set initialized');
    }
  "

  echo "Creating admin user..."
  mongosh --quiet --eval "
    db = db.getSiblingDB('admin');
    if (!db.getUser('admin')) {
      db.createUser({
        user: 'admin',
        pwd: 'admin',
        roles: [{ role: 'root', db: 'admin' }]
      });
      print('Admin user created');
    } else {
      print('Admin user already exists');
    }
  "

  touch "$INIT_FLAG"
  echo "MongoDB initialization complete"
}

wait_for_primary() {
  until /usr/local/bin/mongo-primary.sh; do
    sleep 1
  done
}

wait_mongod() {
  wait "$MONGOD_PID"
}
