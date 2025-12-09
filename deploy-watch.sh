#!/bin/bash

echo "Watching for changes and auto-deploying..."

while true; do
    # Get current commit hash before pulling
    OLD_COMMIT=$(git rev-parse HEAD)
    
    git pull origin main > /dev/null 2>&1
    
    # Get commit hash after pulling
    NEW_COMMIT=$(git rev-parse HEAD)
    
    # Only rebuild if commit changed
    if [ "$OLD_COMMIT" != "$NEW_COMMIT" ]; then
        echo "New changes detected! Building..."
        
        echo "Building backend..."
        go build -o golf-card-game
        
        echo "Building frontend..."
        cd frontend && npm run build && cd ..
        
        # Kill old process and start new one
        echo "Restarting service..."
        pkill -f golf-card-game
        ./golf-card-game &
        
        echo "Deployment complete at $(date)"
    fi
    
    # Wait before checking again (e.g., every 30 seconds)
    sleep 30
done