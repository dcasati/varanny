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
| `rc.d/xara_hf`, `rc.d/graywolf`, `libexec/graywolf-run` | `/usr/local/etc/rc.d/`, `/usr/local/libexec/graywolf/` | Optional: always-on xara for HF and graywolf APRS (see below) |
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

## Optional: graywolf APRS alongside xara

[graywolf](https://github.com/chrissnell/graywolf) can run APRS on VHF with its
own AFSK modem and on HF through xara's KISS port. Its Linux release runs under
the Linuxulator.

* Install `linux-rl9-alsa-lib` and `linux-rl9-alsa-plugins-oss`, unpack the
  release into `/usr/local/libexec/graywolf/`, brand both binaries with
  `brandelf -t Linux`, and install `libexec/graywolf-run` next to them.
* `digirig-link` gives each `pcm.<radio>` an ALSA `hint`, which is what makes
  cpal (and graywolf) list it: use `ft818`/`g90` as the device path.
* Keep graywolf's data out of `/var/db`: `/compat/linux/var/db` shadows it for
  Linux binaries. `rc.d/graywolf` uses `~/.local/share/graywolf`.
* Before listening on `0.0.0.0`, create the admin user so nobody on the LAN can:
  `graywolf auth set-password --user admin -config <dir>/graywolf.db`.
* PTT: method `rigctld`, device `127.0.0.1:<rigctld port>`.
* HF: a KISS-only channel plus a `tcp-client` KISS interface (TNC mode) to
  xara's KISS port. That needs xara running all the time, so enable
  `rc.d/xara_hf`; `bin/xara-varahf` then just waits while varanny clients use
  the same xara.
* One radio can only do one thing: graywolf and a varanny FM session share
  the FT-818.

### graywolf step by step

Run these as root unless noted; `radio` is the account that owns the Digirig
`~/.asoundrc` (replace it with yours).

1. Linux ALSA libraries for graywolf-modem:

   ```sh
   pkg install linux-rl9-alsa-lib linux-rl9-alsa-plugins-oss
   ```

2. Release binaries (check the
   [releases](https://github.com/chrissnell/graywolf/releases) for the latest):

   ```sh
   v=0.14.14
   fetch https://github.com/chrissnell/graywolf/releases/download/v$v/graywolf_${v}_linux_x86_64.tar.gz
   tar -xzf graywolf_${v}_linux_x86_64.tar.gz graywolf graywolf-modem
   install -d /usr/local/libexec/graywolf
   install -m 755 graywolf graywolf-modem /usr/local/libexec/graywolf/
   brandelf -t Linux /usr/local/libexec/graywolf/graywolf
   brandelf -t Linux /usr/local/libexec/graywolf/graywolf-modem
   install -m 555 libexec/graywolf-run /usr/local/libexec/graywolf/
   install -m 555 rc.d/graywolf /usr/local/etc/rc.d/
   ```

3. Check that the Digirigs show up (as `radio`, after `digirig-link` has run):

   ```sh
   /usr/local/libexec/graywolf/graywolf-modem --list-audio
   ```

   Expect `ft818` / `g90` entries described as "Digirig ...".

4. Create the data directory and the admin user *before* exposing the UI
   (as `radio`). The command reads the password twice from stdin:

   ```sh
   d=~/.local/share/graywolf
   mkdir -p $d
   openssl rand -base64 18 > $d/admin-password
   chmod 600 $d/admin-password
   (cat $d/admin-password; cat $d/admin-password) |
       /usr/local/libexec/graywolf/graywolf auth set-password --user admin -config $d/graywolf.db
   ```

5. Enable and start the service:

   ```sh
   sysrc graywolf_enable=YES graywolf_runas=radio
   # optional: graywolf_http=0.0.0.0:8088 graywolf_dir=... graywolf_log=...
   service graywolf start
   service graywolf status
   ```

   The web UI is on `http://<host>:8088/`; log in as `admin`.

6. Configure it, either in the UI or with the REST API (as `radio`):

   ```sh
   d=~/.local/share/graywolf; api=http://127.0.0.1:8088/api; jar=$d/.cookies
   curl -s -c $jar -H 'Content-Type: application/json' \
       -d "{\"username\":\"admin\",\"password\":\"$(cat $d/admin-password)\"}" $api/auth/login
   gw() {
       if [ -n "${3-}" ]; then
           curl -s -b $jar -X "$1" -H "Content-Type: application/json" -d "$3" "$api$2"
       else
           curl -s -b $jar -X "$1" "$api$2"
       fi
       echo
   }

   # station
   gw PUT /station/config '{"callsign":"N0CALL"}'

   # FT-818 Digirig: input (id 1) and output (id 2)
   gw POST /audio-devices '{"name":"FT-818 RX","direction":"input","source_type":"soundcard","device_path":"ft818","sample_rate":48000,"channels":1,"format":"s16le","gain_db":0}'
   gw POST /audio-devices '{"name":"FT-818 TX","direction":"output","source_type":"soundcard","device_path":"ft818","sample_rate":48000,"channels":1,"format":"s16le","gain_db":-12}'

   # channel 1: VHF APRS, 1200 baud AFSK on the FT-818
   gw POST /channels '{"name":"VHF 144.390 (FT-818)","mode":"aprs","input_device_id":1,"input_channel":0,"output_device_id":2,"output_channel":0,"modem_type":"afsk","bit_rate":1200,"mark_freq":1200,"space_freq":2200,"profile":"A","num_slicers":1,"fix_bits":"none","num_decoders":1}'

   # PTT for channel 1 through the FT-818 rigctld
   gw POST /ptt '{"channel_id":1,"method":"rigctld","device_path":"127.0.0.1:4533","slot_time_ms":10,"persist":63}'

   # channel 2: KISS-only (no audio), fed by xara on the G90
   gw POST /channels '{"name":"HF xara (G90)","mode":"aprs","input_device_id":null,"output_device_id":0,"modem_type":"afsk","bit_rate":1200,"mark_freq":1200,"space_freq":2200,"profile":"A","num_slicers":1,"fix_bits":"none","num_decoders":1}'

   # KISS client to xara's KISS port, bound to channel 2
   gw POST /kiss '{"type":"tcp-client","remote_host":"127.0.0.1","remote_port":8100,"channel":2,"mode":"tnc","allow_tx_from_governor":true,"reconnect_init_ms":2000,"reconnect_max_ms":60000}'
   ```

   Ports 4533 (FT-818 rigctld) and 8100 (xara KISS) are the ones used here;
   check yours in `digirig.conf` and `xara.conf`.

7. Keep xara running for the HF channel and let varanny share it:

   ```sh
   install -m 555 rc.d/xara_hf /usr/local/etc/rc.d/
   install -m 555 bin/xara-varahf /usr/local/bin/
   sysrc xara_hf_enable=YES xara_hf_runas=radio
   service xara_hf start
   ```

8. Verify:

   ```sh
   gw GET /channels/1/stats     # rx_frames grows as packets decode
   gw GET '/packets?limit=5'
   gw GET /kiss                 # the xara client should be connected
   tail -f /var/log/graywolf.log
   ```

   If decodes are missing, check the radio's squelch (open it) and the Digirig
   capture level, e.g. `mixer -f /dev/mixer5 mic=0.10`. Use `-f`:
   `mixer -d` changes `hw.snd.default_unit`.

Beacons, digipeater and iGate are off until you configure them in the UI.
