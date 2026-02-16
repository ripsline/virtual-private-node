package welcome

import (
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "strings"

    "github.com/charmbracelet/lipgloss"
    qrcode "github.com/skip2/go-qrcode"

    "github.com/ripsline/virtual-private-node/internal/theme"
)

func (m Model) viewPairing(bw int) string {
    halfW := (bw - 4) / 2
    cardH := theme.BoxHeight

    // Zeus card
    var zeusLines []string
    zeusEnabled := m.cfg.HasLND() && m.cfg.WalletExists()
    if zeusEnabled {
        restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
        status := theme.GreenDot.Render("●") + " ready"
        if restOnion == "" {
            status = theme.RedDot.Render("●") + " waiting"
        }
        zeusLines = []string{
            theme.Lightning.Render("⚡️ Zeus Wallet"), "",
            theme.Dim.Render("LND REST over Tor"), "",
            status, "", theme.Action.Render("Select for setup ▸"),
        }
    } else if m.cfg.HasLND() {
        zeusLines = []string{
            theme.Grayed.Render("⚡️ Zeus Wallet"), "",
            theme.Grayed.Render("Create LND wallet first"),
        }
    } else {
        zeusLines = []string{
            theme.Grayed.Render("⚡️ Zeus Wallet"), "",
            theme.Grayed.Render("Install LND from Add-ons"),
        }
    }
    zBorder := theme.NormalBorder
    if m.pairingFocus == 0 {
        if zeusEnabled {
            zBorder = theme.SelectedBorder
        } else {
            zBorder = theme.GrayedBorder
        }
    }
    zeusCard := zBorder.Width(halfW).Padding(1, 2).Render(padLines(zeusLines, cardH))

    // Sparrow card
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    sStatus := theme.GreenDot.Render("●") + " ready"
    if btcRPC == "" {
        sStatus = theme.RedDot.Render("●") + " waiting"
    }
    sparrowLines := []string{
        theme.Bitcoin.Render("₿ Sparrow Wallet"), "",
        theme.Dim.Render("Bitcoin Core RPC / Tor"), "",
        sStatus, "", theme.Action.Render("Select for setup ▸"),
    }
    sBorder := theme.NormalBorder
    if m.pairingFocus == 1 {
        sBorder = theme.SelectedBorder
    }
    sparrowCard := sBorder.Width(halfW).Padding(1, 2).Render(padLines(sparrowLines, cardH))

    return lipgloss.JoinHorizontal(lipgloss.Top, zeusCard, "  ", sparrowCard)
}

func (m Model) viewZeus() string {
    bw := min(m.width-4, theme.ContentWidth)
    var lines []string
    lines = append(lines, theme.Lightning.Render("⚡️ Zeus Wallet — LND REST over Tor"))
    lines = append(lines, "")
    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    if restOnion == "" {
        lines = append(lines, theme.Warn.Render("Not available yet."))
    } else {
        lines = append(lines, "  "+theme.Label.Render("Type: ")+theme.Mono.Render("LND (REST)"))
        lines = append(lines, "")
        lines = append(lines, "  "+theme.Label.Render("Server address:"))
        lines = append(lines, "  "+theme.Mono.Render(restOnion))
        lines = append(lines, "  "+theme.Label.Render("REST Port: ")+theme.Mono.Render("8080"))
        lines = append(lines, "")
        mac := readMacaroonHex(m.cfg)
        if mac != "" {
            preview := mac[:min(40, len(mac))] + "..."
            lines = append(lines, "  "+theme.Label.Render("Macaroon (Hex format):"))
            lines = append(lines, "  "+theme.Mono.Render(preview))
            lines = append(lines, "")
            lines = append(lines, "  "+theme.Action.Render("[m] full macaroon    [r] QR code"))
        }
    }
    lines = append(lines, "")
    lines = append(lines, theme.Dim.Render("1. download & verify Zeus"))
    lines = append(lines, theme.Dim.Render("2. Advanced Set-Up"))
    lines = append(lines, theme.Dim.Render("3. + Create or connect a wallet"))
    lines = append(lines, theme.Dim.Render("4. Server address, REST Port, Macaroon (Hex format) above"))

    box := theme.Box.Width(bw).Padding(1, 2).Render(strings.Join(lines, "\n"))
    title := theme.Title.Width(bw).Align(lipgloss.Center).Render(" Zeus Wallet Setup ")
    footer := theme.Footer.Render("  m macaroon • r QR • backspace back • q quit  ")
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}

func (m Model) viewSparrow() string {
    bw := min(m.width-4, theme.ContentWidth)
    var lines []string
    lines = append(lines, theme.Header.Render("₿ Sparrow — Bitcoin Core RPC over Tor"))
    lines = append(lines, "")
    lines = append(lines, theme.Warning.Render("WARNING: Cookie changes on restart."))
    lines = append(lines, theme.Warning.Render("WARNING: Reconnect Sparrow after reboot."))
    lines = append(lines, "")
    btcRPC := readOnion("/var/lib/tor/bitcoin-rpc/hostname")
    if btcRPC != "" {
        port := "8332"
        if !m.cfg.IsMainnet() {
            port = "48332"
        }
        cookie := readCookieValue(m.cfg)
        lines = append(lines, "  "+theme.Label.Render("URL:"))
        lines = append(lines, "  "+theme.Mono.Render(btcRPC))
        lines = append(lines, "  "+theme.Label.Render("Port: ")+theme.Mono.Render(port))
        lines = append(lines, "")
        lines = append(lines, "  "+theme.Label.Render("User: ")+theme.Mono.Render("__cookie__"))
        if cookie != "" {
            lines = append(lines, "  "+theme.Label.Render("Password:"))
            lines = append(lines, "  "+theme.Mono.Render(cookie))
        }
    }
    lines = append(lines, "")
    lines = append(lines, theme.Dim.Render("1. download & verify Sparrow Wallet"))
    lines = append(lines, theme.Dim.Render("2. Sparrow → Settings → Server"))
    lines = append(lines, theme.Dim.Render("3. Bitcoin Core tab, enter details above"))
    lines = append(lines, theme.Dim.Render("4. Test Connection"))

    box := theme.Box.Width(bw).Padding(1, 2).Render(strings.Join(lines, "\n"))
    title := theme.Title.Width(bw).Align(lipgloss.Center).Render(" Sparrow Wallet Setup ")
    footer := theme.Footer.Render("  backspace back • q quit  ")
    full := lipgloss.JoinVertical(lipgloss.Center, "", title, "", box, "", footer)
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}

func (m Model) viewQR() string {
    restOnion := readOnion("/var/lib/tor/lnd-rest/hostname")
    mac := readMacaroonHex(m.cfg)
    if restOnion == "" || mac == "" {
        return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
            theme.Warn.Render("QR not available."))
    }
    uri := fmt.Sprintf("lndconnect://%s:8080?macaroon=%s",
        restOnion, hexToBase64URL(mac))
    qr := renderQRCode(uri)
    var lines []string
    lines = append(lines, theme.Dim.Render("Zoom out: Cmd+Minus / Ctrl+Minus"))
    if qr != "" {
        lines = append(lines, qr)
    }
    lines = append(lines, theme.Footer.Render("backspace back • q quit"))
    return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
        lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func renderQRCode(data string) string {
    qr, err := qrcode.New(data, qrcode.Low)
    if err != nil {
        return ""
    }
    bm := qr.Bitmap()
    rows, cols := len(bm), len(bm[0])
    var b strings.Builder
    for y := 0; y < rows; y += 2 {
        for x := 0; x < cols; x++ {
            top := bm[y][x]
            bot := y+1 < rows && bm[y+1][x]
            switch {
            case top && bot:
                b.WriteString("█")
            case top:
                b.WriteString("▀")
            case bot:
                b.WriteString("▄")
            default:
                b.WriteString(" ")
            }
        }
        if y+2 < rows {
            b.WriteString("\n")
        }
    }
    return b.String()
}

func hexToBase64URL(hexStr string) string {
    data, err := hex.DecodeString(hexStr)
    if err != nil {
        return ""
    }
    return base64.RawURLEncoding.EncodeToString(data)
}