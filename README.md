# Shlyuz client

Public command-line client for Shlyuz tunnels. The server, dashboard and
deployment infrastructure are intentionally not included in this repository.

After installation, authenticate with the token from the dashboard:

    tunnel login --token YOUR_TOKEN --server tunel.ibodov.uz:7000

Open an HTTPS/WebSocket tunnel:

    tunnel http 3000 --subdomain my-app

Open a TCP/MQTT tunnel:

    tunnel tcp 1883
