#!/bin/sh

# Start Xvfb (virtual display)
Xvfb :99 -screen 0 1920x1080x24 &
sleep 1

# Start fluxbox (window manager)
fluxbox -display :99 &
sleep 1

# Start x11vnc (VNC server)
x11vnc -display :99 -forever -shared -rfbport 5900 -nopw &
sleep 1

# Start noVNC (web-based VNC client)
websockify --web=/usr/share/novnc/ 6080 localhost:5900 &
sleep 1

# Start mcp-service
./mcp-service
