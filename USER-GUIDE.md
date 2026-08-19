# SaturX — User guide

A short overview of what you need and how to run the app.

---

## What you need

1. **The app folder** should contain:
   - **SaturX.exe** (Windows) or **saturx** (Mac/Linux) — main application;
   - optionally **viewerbot.exe** / **viewerbot** (if included in your package);
   - a **`.env`** file (create and fill it in as described below).

2. **License key** — you get this from the seller after purchase. Enter it once in the web UI on first launch.

3. **Kick OAuth app** — so the app can post to chat on your behalf:
   - go to [developers.kick.com](https://developers.kick.com/);
   - create an application;
   - set **Redirect URL** to: `http://localhost:8080/oauth/callback` (same port as dashboard, or `http://localhost:<DASHBOARD_PORT>/oauth/callback`);
   - enable **chat:write** (and **channel:read** if needed);
   - copy **Client ID** and **Client Secret** into your `.env` file.

---

## `.env` setup

Create a file named **`.env`** (leading dot) in the same folder as the executable. Example:

```env
KICK_CLIENT_ID=your_client_id_from_developers.kick.com
KICK_CLIENT_SECRET=your_client_secret
CHANNEL_SLUG=your_channel_name
```

- **CHANNEL_SLUG** is the channel segment in the Kick URL. For `kick.com/your_channel`, use `your_channel`.
- Optional: `DASHBOARD_PORT=8080` (web dashboard port; default is 8080).

**Important:** you do **not** need to put the license server URL or internal signing keys in `.env` — those are already embedded in the build.

---

## Running the app

1. Start the program:
   - **Windows:** double-click **SaturX.exe**, or open a terminal in that folder and run `SaturX.exe`;
   - **Mac/Linux:** in a terminal, from the app folder: `./saturx`.

2. Open **http://localhost:8080** in your browser (or the port set in `DASHBOARD_PORT`).

3. On first visit you should see **Enter license key**. Paste the key from the seller and confirm. After activation, the main UI opens.

4. If no Kick accounts are linked yet, use **Add account** (or the equivalent) in the dashboard. Complete Kick login and authorize the app. The account appears in the list; you can send messages to the configured channel.

---

## Web dashboard

- **Chat** — live view of your Kick channel chat.
- **Stream** — stream player when the channel is live.
- **Accounts** — linked Kick accounts and online/offline status.
- **Send messages** — pick an account, type a message, send.

Messages go through Kick’s official API (OAuth). You do not need to install Python or any runtime for the main app.

---

## Message presets (`messages.txt`)

**What it does:** a plain text file next to **SaturX.exe** named **`messages.txt`** defines **preset messages** for chat. Each **non-empty line** is one message. The dashboard shows them as a row of buttons labeled **Presets (messages.txt):** — click a button to **send that line to chat** immediately using the **account currently selected** in the bottom bar (same as manual send).

**How to create or edit**

1. Open (or create) **`messages.txt`** in the **same folder** as the executable (UTF-8 is fine).
2. Put **one message per line**. Empty lines are ignored; leading/trailing spaces on each line are trimmed.
3. Keep each line within Kick’s limit (**up to 500 characters** per message).
4. Save the file. **Reload the dashboard** in the browser (**F5** or refresh) so the preset buttons update — the app reads the file when the accounts panel loads.

If **`messages.txt`** is missing or only has blank lines, the preset bar stays hidden.

**Auto-send (same file):** the **Auto-send from selected account** row uses **`messages.txt` too**. When you turn it **On** and set an interval (seconds), SaturX sends the lines **in order**, one per tick, rotating through the file (line 1, then 2, … then back to 1). Only the **currently selected** account is used. If the file is empty, auto-send does nothing useful until you add lines.

---

## Viewerbot (if included)

If your package includes **viewerbot** or **viewerbot.exe**:

- Run it **separately** from SaturX, following the seller’s instructions (channel or options may be in the same `.env` or another config).
- It is a standalone executable; you do not need Python installed.

---

## Multiple accounts and proxies

- You can add several Kick accounts in the dashboard and switch between them.
- **Proxies are set in the dashboard**, in the **Accounts** panel: each account has its own **Proxy** field and **Save (✓)** button. Enter the proxy string for that account only—there is no single global proxy for all accounts in `.env`.

### Supported proxy format (SaturX chat / API)

Proxies must be **SOCKS5**. Each proxy is a **single line** with **four fields** separated by colons:

```text
host:port:username:password
```

Examples:

- With authentication: `proxy.example.com:1080:myuser:mypass`
- No login (empty user and password): `proxy.example.com:1080::`  
  (keep both trailing colons so there are still four fields)

**Notes:**

- Use a **hostname or IPv4** for **host**; IPv6 literals (e.g. `[::1]`) are not supported in this string format.
- **HTTP / HTTPS proxies are not supported** for this field—only SOCKS5.
- If the password contains **`:`** characters, everything after the third `:` counts as the password (only the first three colons separate host, port, and username).
- Set the proxy in the **Accounts** section of the dashboard (per account), or use a **`.kick_proxies`** file in the app folder: **one line per account**, in the same order as your accounts (line 1 → first account, line 2 → second, …). On startup, empty proxy fields are filled from that file. Blank lines are skipped.

---

## FAQ

| Question | Answer |
|----------|--------|
| Where do I get Client ID and Client Secret? | [developers.kick.com](https://developers.kick.com/) → create an app → copy values into `.env`. |
| Where do I get the license key? | From the seller after purchase. Enter once in the browser at http://localhost:8080. |
| Do I need to set the license server URL? | No. It is embedded in the application. |
| App says “Set KICK_CLIENT_ID…” | Create `.env` next to the executable and set `KICK_CLIENT_ID`, `KICK_CLIENT_SECRET`, and `CHANNEL_SLUG`. |
| http://localhost:8080 does not load | Make sure the app is running and that antivirus/firewall is not blocking it. |
| What proxy format does SaturX use? | SOCKS5 only, as `host:port:username:password` (see **Multiple accounts and proxies**). |
| What is `messages.txt`? | Preset chat lines (one per line) next to the exe; buttons in the dashboard + optional auto-send. See **Message presets (`messages.txt`)**. |

For license or access issues, contact the seller or support.
