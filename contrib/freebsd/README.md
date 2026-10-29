# varanny on FreeBSD with xara and Digirigs

A headless FreeBSD station where RadioMail (or any VARA client) discovers and
launches [xara](https://github.com/x0rro/xara), a VARA-compatible HF/FM modem,
through `varanny`. Each radio has its own Digirig, and every Digirig is tied
1:1:1 to its sound card and its radio, so several radios can be connected at
once.

Tested on FreeBSD 15.1 amd64 with a Xiegu G90 (HF) and a Yaesu FT-818ND (FM),
each on its own Digirig.

## How the pieces fit

```
RadioMail ──DNS-SD──> varanny :8273 ──start/stop──> xara-varahf / xara-varafm
                                                       │ audio: ALSA pcm.<radio> ─> OSS /dev/dspN
                                                       │ PTT:   rigctld :453x ─> /dev/cuaUN
digirig.conf ──> digirig-link (devd) ──> /var/run/digirig/asound.conf (pcm.<radio>)
            └──> rc.d/rigctld ──> one rigctld per radio (via rigctld-run)
```

| Radio | ALSA device | rigctld | xara cmd/data | KISS | xara web UI |
|---|---|---|---|---|---|
| G90 (HF) | `g90` | 4532 | 8300/8301 | 8100 | 8090 |
| FT-818ND (FM) | `ft818` | 4533 | 8310/8311 | 8110 | 8190 |

### Why a Digirig can be mapped reliably

A Digirig is a small USB hub carrying a CM108 sound chip (`uaudio` → `pcmN`)
and a CP2102N serial chip (`uslcom` → `/dev/cuaUN`). The `pcm` and `cuaU`
unit numbers change with plug order, and the CM108 has no serial number, but
the CP2102N does, and both chips share the same parent hub:

```
dev.uslcom.0.%pnpinfo: ... sernum="5a1d10d4..."
dev.uslcom.0.%parent: uhub4
dev.uaudio.0.%parent: uhub4
dev.pcm.5.%parent:    uaudio0      -> ft818 is /dev/dsp5
```

So the CP2102N serial number identifies the radio, its serial port and its
sound card.

## Files

| File in this directory | Install to | Purpose |
|---|---|---|
| `etc/digirig.conf` | `/usr/local/etc/digirig.conf` | One line per radio: Digirig serial, hamlib model, baud, rigctld port |
| `libexec/digirig-link` | `/usr/local/libexec/digirig-link` | Writes `/var/run/digirig/asound.conf` with `pcm.<radio>` for each connected Digirig |
| `libexec/rigctld-run` | `/usr/local/libexec/rigctld-run` | Finds the CP2102N by serial number and execs `rigctld` on its current `/dev/cuaUN` |
| `devd/digirig-link.conf` | `/usr/local/etc/devd/digirig-link.conf` | Re-runs `digirig-link` when a `pcm` or `uslcom` device attaches or detaches |
| `rc.d/rigctld` | `/usr/local/etc/rc.d/rigctld` | One `rigctld` per radio in `digirig.conf` |
| `rc.d/varanny` | `/usr/local/etc/rc.d/varanny` | Runs `varanny` as the station user |
| `bin/xara-varahf`, `bin/xara-varafm` | `/usr/local/bin/` | Wrappers: varanny passes `Args` as one argument, xara needs several |
| `libexec/xara-varanny` | `/usr/local/libexec/xara-varanny` | Runs xara for the wrappers and holds varanny's SIGTERM until an open link has disconnected (max 20 s) |
| `etc/varanny/xara-hf.ini`, `xara-fm.ini` | `/usr/local/etc/varanny/` | Minimal VARA ini: varanny reads the port to advertise from here |
| `xara/xara-hf.conf`, `xara-fm.conf` | `/usr/local/etc/xara/` (or `~/.config/xara/`) | xara settings per radio |
| `xara/asoundrc` | `~/.asoundrc` of the user running varanny | Includes the generated `pcm.<radio>` definitions |
| `../../sample-configs/varanny.freebsd.json` | `/usr/local/etc/varanny/varanny.json` | Advertises both modems |

## Setup

1. Packages and builds:
   ```sh
   pkg install go rust hamlib alsa-lib alsa-plugins
   # varanny (from this repo)
   CGO_ENABLED=1 go build && install -m 755 varanny /usr/local/bin/
   # xara (registered build, needed for --registered in the wrappers)
   cargo build --release -p xara --features registered
   install -m 755 target/release/xara target/release/xara-monitor /usr/local/bin/
   ```

2. Find each Digirig's CP2102N serial number (plug them in one at a time):
   ```sh
   sysctl dev.uslcom | grep -o 'sernum="[^"]*"'
   ```
   and put it in `/usr/local/etc/digirig.conf` with the radio's hamlib model
   (`rigctl -l`), its CAT baud rate and a unique rigctld port.

3. Install the files from the table above, then set the station user
   (who owns `~/.asoundrc`) and enable the services:
   ```sh
   sysrc rigctld_enable=YES varanny_enable=YES varanny_runas=radio
   service devd restart
   service rigctld start
   service varanny start
   ```

4. Check the mapping:
   ```sh
   cat /var/run/digirig/asound.conf
   service rigctld status
   printf 'f\nm\nt\n' | rigctl -m 2 -r 127.0.0.1:4533   # freq, mode, PTT=0
   ```

5. In RadioMail, add the modems that varanny announces.

## Adding a radio

Add a line to `digirig.conf`, give it free ports in a new xara config, a
wrapper and ini file, and a `Modems` entry in `varanny.json`. Then
`service rigctld restart` and `service varanny restart`. The new
`pcm.<radio>` appears as soon as its Digirig is plugged in.

## Operating notes

* `service rigctld status|restart [radio]` acts on one radio or all of them.
  An instance whose Digirig is unplugged keeps retrying every 5 s and picks
  the radio up when it returns.
* `rigctld` opens the port with `-C rts_state=OFF,dtr_state=OFF`, so opening
  it does not key a radio wired for RTS PTT.
* xara only transmits into a numbered OSS unit (`/dev/dspN`), never into
  `/dev/dsp`, which follows `hw.snd.default_unit`. That is why
  `digirig-link` writes the resolved unit rather than a symlink.
* FreeBSD virtual channels let several programs record from one Digirig, so
  `xara-monitor --audio alsa --capture g90` (VARA HF only) can run next to
  xara.
* RadioMail asks varanny to stop the modem right after it asks xara to
  disconnect, and varanny stops it with SIGTERM. If xara exits then, the
  other station never sees the link close and keeps retrying.
  `xara-varanny` holds the SIGTERM until xara reports `DISCONNECTED`
  (at most 20 s).
* The wrappers use `--addr 0.0.0.0` so remote clients can reach xara; its
  command, data and KISS ports are unauthenticated, so keep them on a trusted
  LAN.
* `varanny`'s `monitor` command picks the input by device name, and every
  CM108 Digirig has the same name, so it can't tell two Digirigs apart.
* Pat Winlink: point `varahf` at `localhost:8300` and `varafm` at
  `localhost:8310`.

### FT-818ND settings

* Menu 14 CAT RATE: match `digirig.conf` (38400 here).
* Mode PKT with menu 25 PKT RATE 1200 for narrow VARA FM (rear DATA jack).
* Menu 40 PKT MIC sets TX level; start around 50 with `drive-db = -10`.
