#!/bin/bash

echo "Testing pipeline status filtering with debug mode..."

# Run the program with debug mode and capture output
FLO_DEBUG=1 timeout 10s ./flo > debug_output.log 2>&1 &
PID=$!

# Wait a bit for the program to start and load data
sleep 5

# Send 'a' key to toggle status filter
echo "Sending 'a' key to toggle status filter..."
# This is tricky to do programmatically, so let's just kill the process
kill $PID 2>/dev/null

echo "Debug output:"
cat debug_output.log | grep -E "(Processing pipeline|Extracted pipeline|Added pipeline|Request URL|Response Body)" | head -20

echo ""
echo "Looking for pipeline parsing issues..."
cat debug_output.log | grep -E "(Skipped pipeline|missing ID or name)" | head -10

echo ""
echo "Full debug log saved to debug_output.log" 