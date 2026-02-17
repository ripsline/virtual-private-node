## Syncthing Setup Guide

This guide walks you through connecting your node's Syncthing
instance with your local device for automatic LND channel backups.

### What this does

Syncthing watches your LND `channel.backup` file and automatically
syncs it to your local device over Tor. If your VPS dies, you can
recover your Lightning channels using this backup and your seed phrase.

No cloud services. No trust. Direct device-to-device sync over Tor.

### Prerequisites

- Virtual Private Node with LND and Syncthing installed
- Syncthing installed on your local device
- Tor Browser installed on your local device

### Step 1: Get your node's Syncthing credentials

From the dashboard, navigate to the **Add-ons** tab and select the
Syncthing card. You will see:

- **URL** — your node's Syncthing onion address
- **User** — `admin`
- **Pass** — your generated password

Press `[u]` to view the full Tor Browser URL.

### Step 2: Access your node's Syncthing web UI

1. Open **Tor Browser**
2. Paste the full URL from the dashboard
3. Login with the user and password from the dashboard

### Step 3: Get your node's Device ID

In the Syncthing web UI on your node:

1. Click **Actions** (top right) → **Show ID**
2. Copy the Device ID (long alphanumeric string)
3. Keep this page open

### Step 4: Install Syncthing on your local device

Download from [syncthing.net](https://syncthing.net/downloads/)
or install via your package manager:

### Step 5: Add your node as a remote device

In your **local** Syncthing web UI (`http://127.0.0.1:8384`):

1. Click **Add Remote Device**
2. Paste your node's Device ID from Step 3
3. Device Name:`rlvpn` (or whatever you prefer)


5. Click **Save**

### Step 6: Get your local Device ID

In your **local** Syncthing web UI:

1. Click **Actions** → **Show ID**
2. Copy your local Device ID

### Step 7: Add your local device to your node

In your **node's** Syncthing web UI (via Tor Browser):

1. Click **Add Remote Device**
2. Paste your local Device ID from Step 6
3. Device Name:`local` (or whatever you prefer)
4. Click **Save**

### Step 8: Share the backup folder

In your **node's** Syncthing web UI:

1. Click **Add Folder**
2. Folder Label:`lnd-backup`
3. Folder Path:`/var/lib/syncthing/lnd-backup`
4. Under the **Sharing** tab, check your local device
5. Under **File Versioning**, select **Simple File Versioning**
    - Keep Versions:`5`

6. Click **Save**

### Step 9: Accept the folder on your local device

In your **local** Syncthing web UI:

1. A notification will appear: "rlvpn wants to share folder lnd-backup"
2. Click **Add**
3. Choose a local folder path (e.g.,`~/lnd-backup`)
4. Click **Save**

### Step 10: Verify

After a few minutes, check your local folder. You should see:

```
~/lnd-backup/channel.backup
```

This file updates automatically whenever your LND channel state
changes. Open a channel, close a channel — the backup syncs.

### Important notes

- **Seed phrase is still required.** Syncthing backs up channel
  state, not your seed. Store your seed separately and securely.
- **Both devices must be online** for sync to occur. Syncthing
  syncs when both peers are connected.
- **The sync runs over Tor.** It may take a minute for devices to
  discover each other through the onion addresses.
- **channel.backup is encrypted.** It's useless without your seed
  phrase. Even if someone intercepts it, they cannot steal funds.
- **Discovery is disabled.** Your node's Syncthing does not announce
  itself to any discovery servers. The connection is direct between
  your two devices via Tor.