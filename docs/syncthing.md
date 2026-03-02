## Syncthing Setup Guide

Syncthing automatically syncs your LND `channel.backup` file from
your VPS to your local device. If your VPS dies, you can recover
your channels with your 24-word seed phrase and this backup file.

Syncthing encrypts all connections using mutual TLS authentication.
Only devices you explicitly approve can connect. The `channel.backup`
file is useless without your seed phrase.

### Requirements

- Syncthing installed on your local computer
- Your VPS has Syncthing installed from the Add-ons tab

### Step 1 — Install Syncthing on Your VPS

1. SSH into your VPS as `ripsline`
2. Go to **Add-ons** tab
3. Select **Syncthing** and press Enter to install

### Step 2 — Install Syncthing on Your Computer

Download Syncthing for your operating system from
[syncthing.net](https://syncthing.net/downloads/).

- **macOS:** Download the `.dmg` or `brew install syncthing`
- **Windows:** Download the installer
- **Linux:** Install from your package manager

Start Syncthing. It opens a web UI at `http://127.0.0.1:8384`.

### Step 3 — Get Your Local Device ID

In your local Syncthing web UI:

1. Click **Actions** (top right)
2. Click **Show ID**
3. Copy the Device ID (looks like `XXXXXXX-XXXXXXX-XXXXXXX-...`)

### Step 4 — Pair Your Device on the VPS

In the VPS dashboard:

1. Go to **Add-ons** tab
2. Select **Syncthing**
3. Press **a** to pair a device
4. Paste your local Device ID
5. Press Enter

The VPS adds your device and shares the backup folder
automatically.

### Step 5 — Add the VPS in Your Local Syncthing

In your local Syncthing web UI:

1. Click **Add Remote Device**
2. Paste the **VPS Device ID** shown on the Syncthing details
   screen in the dashboard
3. Under **Addresses**, replace `dynamic` with: `tcp://<your-server-ip>:22000`
4. Click **Save**

### Step 6 — Accept the Backup Folder

After the devices connect, your local Syncthing will prompt you
to accept a shared folder called `lnd-backup`.

1. Click **Accept** (or **Add**)
2. Choose a local folder path, for example:
    - macOS: `~/lnd-backup`
    - Windows: `C:\Users\YourName\lnd-backup`
    - Linux: `~/lnd-backup`
3. Set the folder to **Receive Only**
4. Click **Save**

### Done

Your `channel.backup` file will sync automatically whenever:

- Your LND channel state changes (open, close, update)
- Your local device is online and connected

The sync happens in seconds. You don't need to keep your computer
on all the time — Syncthing will catch up the next time both
devices are online.

### Verify It's Working

Check that `channel.backup` appears in your local folder:

- **macOS/Linux:** `ls ~/lnd-backup/`
- **Windows:** Open the folder in Explorer

In the VPS dashboard, the Syncthing service should show a green
dot on the Dashboard tab.

### Recovery

If your VPS is lost:

1. Get a new VPS and run the bootstrap script
2. Install LND from the Dashboard
3. Instead of creating a new wallet, recover with your 24-word
   seed phrase: `lncli create` → select recover
4. Provide your `channel.backup` file when prompted
5. LND will force-close all channels and recover your funds

### Security

- Syncthing uses mutual TLS authentication — only devices you
  approve can connect
- The sync port (22000) rejects all unapproved devices
  immediately
- The `channel.backup` file is encrypted by LND and useless
  without your 24-word seed phrase
- Discovery and relay servers are disabled — your device connects
  directly to the VPS by IP address
- The Syncthing web UI on the VPS is only accessible via Tor
  (not exposed to clearnet)

### Troubleshooting

**Devices not connecting:**

- Verify both devices are running (green dot in web UI)
- Check that the VPS address is correct:
  `tcp://<vps-ip>:22000`
- Check firewall: `sudo ufw status` should show port 22000 open

**Folder not syncing:**

- Check that the folder is shared with both devices
- VPS side should be **Send Only** or **Send & Receive**
- Local side should be **Receive Only**
- Check Syncthing logs: **Actions → Logs** in the web UI

**Web UI access on VPS:**

The Syncthing web UI is available over Tor for advanced
configuration. The onion address, username, and password are shown
on the Syncthing details screen in the dashboard. Use Tor Browser
to access it.