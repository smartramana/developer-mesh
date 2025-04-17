#!/bin/bash

# Run AI Agent simulation tests

# Create a virtual environment
echo "Setting up virtual environment..."
python3 -m venv venv
source venv/bin/activate

# Install required dependencies
echo "Installing required dependencies..."
pip install requests numpy

# Make the test script executable
chmod +x ai_agent_simulation.py

# Run the tests
echo "Running AI Agent simulation tests..."
python ai_agent_simulation.py

# Save the exit code
EXIT_CODE=$?

# Display result based on exit code
if [ $EXIT_CODE -eq 0 ]; then
    echo "All tests passed!"
else
    echo "Some tests failed."
fi

exit $EXIT_CODE
