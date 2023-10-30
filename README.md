# varanny

`varanny` is an enhancement tool for VARA, a widely used software modem in amateur radio digital transmissions. VARA functions through two TCP ports - one for commands, the other for data. This TCP/IP interaction enables the VARA modem software to run on a radio-connected, headless computer, while client applications operate on a separate network device. However, since VARA wasn't designed to function as a service, this setup comes with certain limitations.

`varanny` steps in to address these limitations, acting as a 'nanny' for VARA. It offers two primary capabilities:

```
┌────────────────────┐                ┌──────────────────────────┐                  ┌────────────────────┐   
│ ┌────────────────┐ │                │      ┌──────────────┐    │                  │                    │   
│ │                │ │                │      │              │    │                  │                    │   
│ │                │ │                │      │ ctrl         │    │                  │                    │   
│ │                ├─┼────port 4532───┼──────▶              │    │                  │       Radio        │   
│ │                │ │                │      │   varanny    │    │                  │                    │   
│ │                │ │        ┌───────┼──────┤              │    │                  │                    │   
│ │                │ │        │       │      │              │    │                  │                    │   
│ │                │ │        │       │      │    manage    │    │                  └──────┬─┬─┬─┬───────┘   
│ │                │ │        │       │      └──┬─────────┬─┘    │                         │ │ │ │           
│ │                │ │        │       │         │         │      │                         │ │ │ │ TX        
│ │                │ │     DNS-SD     │         │         │      │                Headset  │ │ │ │ RX        
│ │                │ │     Service    │         │         ▽      │                 Jack    │ │ │ │ PTT       
│ │                │ │  Announcement  │         │  ┌ ─ ─ ─ ─ ─   │                         │ │ │ │ GND       
│ │                │ │        │       │         │    VARA.ini │┐ │                         │ │ │ │           
│ │   RadioMail    │ │        │       │         │  └ ─ ─ ─ ─ ─   │                         │ │ │ │           
│ │                │ │        │       │         │    └ ─ ─ ─ ─ ┘ │                  ┌──────┴─┴─┴─┴───────┐   
│ │                │ │        ▽       │         ▽                │                  │                    │   
│ │                │ │                │      ┌─────────────┐     │                  │   ┌────────────┐   │   
│ │                ◀─┼──CMD port 8300─┼──────▶             ├─────┼────Audio out─────┼───▶            │   │   
│ │                │ │                │      │    VARA     │     │                  │   │ Soundcard  │   │   
│ │                ◀─┼─DATA port 8301─┼──────▶             ◀─────┼─────Audio in─────┼───┤            │   │   
│ │                │ │                │      └─────────────┘     │                  │   └────────────┘   │   
│ │                │ │                │                          │                  │                    │   
│ │                │ │                │      ┌─────────────┐     │                  │   ┌────────────┐   │   
│ │                │ │                │      │             │     │                  │   │            │   │   
│ │                ◀─┼───port 8273────┼──────▶   rigctld   │─────┼──────usb─────────┼───▶    PTT     │   │   
│ │                │ │                │      │             │     │                  │   │            │   │   
│ │                │ │                │      └─────────────┘     │                  │   └────────────┘   │   
│ └────────────────┘ │                │                          │                  │                    │   
│                    │      WiFi      │                          │       USB        │                    │   
│    iPhone (iOS)    │                │    Headless Computer     │                  │ Sound Card Adapter │   
└────────────────────┘                └──────────────────────────┘                  └────────────────────┘   
```

## Service Announcement
`varanny` announces the service through DNS Service Discovery, enabling clients to discover an active VARA instance and automatically retrieve the IP and command port configured for that instance. In VARA modems, the data port number is always one more than the command port number.

The service are broadcasted as `_varafm-modem._tcp` and `_varahf-modem._tcp` depending on the modem type and contain a TXT entry with `;` separated options.

### Supported options
* `launchport=` port of varanny launcher. Present if there is a cmd specified in the configuration file.
* `catport=` port of the cat control daemon, if any.
* `catdialect` type of cat control daemon. Currently only `hamlib` is supported.

To test if the service is running, you can validate from a terminal on macOS

```
$ dns-sd -B _varahf-modem._tcp
Browsing for _varahf-modem._tcp
DATE: ---Tue 24 Oct 2023---
18:20:18.020  ...STARTING...
Timestamp     A/R    Flags  if Domain               Service Type         Instance Name
18:20:18.021  Add        3   1 local.               _varahf-modem._tcp.  VARA HF Modem
18:20:18.021  Add        3   6 local.               _varahf-modem._tcp.  VARA HF Modem
18:20:18.021  Add        2   7 local.               _varahf-modem._tcp.  VARA HF Modem
```

and resolve the service address with

```
$ dns-sd -L "VARA HF Modem" _varahf-modem._tcp local
Lookup VARA HF Modem._varahf-modem._tcp.local
DATE: ---Tue 24 Oct 2023---
18:21:15.325  ...STARTING...
18:21:15.326  VARA\032HF\032Modem._varahf-modem._tcp.local. can be reached at cervin.local.local.:8400 (interface 1) Flags: 1
 launchport=8273\; catport=4532\; catdialect=hamlib\;
```

The service accnouncment has been inspired by https://github.com/hessu/aprs-specs/blob/master/TCP-KISS-DNS-SD.md

## Remote Management
`varanny` allows client applications to remotely start and stop the VARA program. This is particularly useful in headless applications, especially when VARA FM and VARA HF share the same sound card interface. Furthermore, VARA, when running on a *nix system via Wine, fails to rebind to its ports after a connection is closed. This means that the VARA application must be restarted after each connection, and `varanny` facilitates this process. This particular issue has been discussed in this [thread](https://groups.io/g/VARA-MODEM/topic/lunchbag_portable_hf_mail/97360073).

### Supported commands
Connections to `varanny` are session-oriented. A client connects, requests to start a modem, performs some operations, and then stops it. Once the modem is stopped, `varanny` will close the connection and restore the VARA configuration file if necessary.

* `start <modem name>` - Starts the modem and rig control defined for `<modem name>`
* `stop` - Stops the processes and close the connection
* `version` - Returns varanny version

### Multiple Configurations
VARA doesn't offer command line configuration options. Therefore, changes like sound card name, PTT com port, etc., need to be made through its GUI. `varnnay` can help manage multiple configurations for you. It automatically swaps the `.ini` configuration file that VARA reads, allowing for seamless configuration changes before each session and restoring the default settings afterward. To create a new configuration, follow these steps:  

1. Open the VARA application. 
2. Adjust the parameters to your preference. 
3. Close the application. 
4. Go to the VARA installation directory (`C:\VARA` or `C:\VARA FM` by default) and duplicate the `VARA.ini` or `VARAFM.ini` file. 

It's a good idea to use a naming convention that reflects what the configuration file represents. For instance, you might name a configuration set up for a Digirig sound card `VARA.digirig.ini`. You can then specify the configuration to use in the `varanny` config file.

## Installation
To set up `varanny`:

1. [Download](https://github.com/islandmagic/varanny/releases/latest) the zip file suitable for your platform.
1. Extract the zip into a local directory.
1. Launch the `varanny` executable. You could either place a shortcut in the startup folder or write a script to start it at boot time.

## Configuration
To configure `varanny`, edit the `varanny.json` file as per your needs. If you do not want the VARA executable to be managed by `varanny`, leave the cmd path field empty ("").

### Configuration Attributes
Configuration must be a valid `.json` file. You can define as many "modems" as you'd like. This can be handy if you use the same computer to connect to multiple radios that require different configurations. If you need to run VARA with different parameters, you can simply clone your VARA directory and configure each copy as necessary.

* `Port` port that varanny agent binds to.
* `Modems` arrray containing modem definitions.
* `Name` name the modem will be advertised under. Must be unique.
* `Type` type of VARA modem, `fm` or `hf`.
* `Cmd` fully quality path to executable to start this VARA modem.
* `Args` arguments to pass the executable.
* `Config` optional path to a VARA configuration file. If present, a backup of the existing VARA.ini or VARAFM.ini file is created and then the specified configuration file is applied. Once the session concludes, the original .ini file is restored. This feature ensures the preservation of original settings while enabling different configurations for specific setups such as a sound card name. Note that the configuration file must be in the same directory as the original VARA.ini or VARAFM.ini files.
* `Port` command port defined in the VARA modem application. 
* `CatCtrl` option cat control definition.
* `Port` port used by cat control agent.
* `Dialect` protocol used by cat control agent. Currently only `hamlib` is supported.
* `Cmd` fully quality path to executable to start the cat control agent.
* `Args` arguments to pass the executable.

[Sample Configuration](https://github.com/islandmagic/varanny/blob/master/varanny.json)

### Running with Wine on Linux
Ensure VARA is installed in its default location and wine executable is in the PATH.

```
{
  "Port": 8273,
  "Modems": [
    {
      "Name": "VARA FM Modem",
      "Type": "fm",
      "Cmd": "wine",
      "Args": "C:\\VARA FM\\VARAFM.exe",
      "Port": 8300,
      "CatCtrl": {
        "Port": 4532,
        "Dialect": "hamlib"
      }    
    },
    {
      "Name": "VARA HF Modem",
      "Type": "hf",
      "Cmd": "wine",
      "Args": "C:\\VARA\\VARA.exe",
      "Port": 8400,
      "CatCtrl": {
        "Port": 4532,
        "Dialect": "hamlib"
      }    
    }
  ]
}
```
