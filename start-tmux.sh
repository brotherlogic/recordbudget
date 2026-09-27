#!/bin/bash

# Ensure the 'recordbudget' session exists
if ! tmux has-session -t recordbudget 2>/dev/null; then
  # Create a new session named 'recordbudget', detached
  cd /workspaces/recordbudget
  tmux new-session -d -s recordbudget
  
  # Split the window horizontally (-h)
  # The left pane will remain a terminal
  # The right pane will run 'gh dash'
  tmux split-window -h -t recordbudget "gh dash"
fi
