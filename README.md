# FEAR Online Server

A replacement backend for the **F.E.A.R. Online**.

The project reproduces the original server-side protocol closely enough
for the retail client to authenticate, enter the lobby, manage account
data, create rooms, and play matches without the original online
service.

**Note**: for now we need `ZLauncher.dll` from dlls_stubs to bypass auth. (You can find it on release page)

For implementation status, protocol findings, and known limitations, see
[PROGRESS.md](PROGRESS.md).

## Requirements

- Go 1.17 or newer
- MongoDB
- F.E.A.R Online on Steam
- ZLauncher.dll stub

## Running the server

From the repository root:

``` bash
cd server
go run ./cmd/fear-online-server
```

Run the test suite with:

``` bash
go test ./...
```

## Client configuration

The client must be pointed at the machine running the replacement
server. There are two addresses to keep in sync: the launcher HTTP
endpoint and `LOGIN_SERVER_IP` used by the game services.

### Local machine

If the client and server run on the same machine, edit:

``` text
LauncherData/Config.xml
```

Set `LauncherUrl` to:

``` text
http://127.0.0.1:8080/LivePatch/Launcher/
```

The login server address can also use `127.0.0.1` when everything is
running locally.

### LAN / multiple PCs

When another computer needs to connect, do not use `127.0.0.1`: loopback
always refers to the machine running the client.

Find the LAN address of the server machine, for example:

``` text
192.168.1.50
```

Then configure the launcher with that address:

``` text
http://192.168.1.50:8080/LivePatch/Launcher/
```

Set `LOGIN_SERVER_IP` to the same reachable address:

``` text
LOGIN_SERVER_IP = 192.168.1.50
```

Replace the example with the actual address of the server machine.

If the launcher works locally but another PC cannot connect, check the
host firewall and make sure the service is listening on an interface
reachable from the LAN rather than only on loopback.

### Internet / remote clients

For clients outside the local network, the configured server address
must be reachable from those clients. NAT, firewall rules, and any
required port forwarding must be configured separately. 

## Launcher and LivePatch

The HTTP service exposes launcher and patch endpoints under:

``` text
/LivePatch/
```

The launcher entry point used by the current client configuration is:

``` text
/LivePatch/Launcher/
```

Steam SSO login is handled through:

``` text
/dialog/oauth/authorize
```

The implementation follows the session-ticket flow expected by the
client rather than the old manual login form.

## Game data catalogs

Decoded catalog files live in:

``` text
internal/catalog/data/
```

The 18 catalog tables are extracted from:

``` text
FEAR_Online/[a-r]3.Arch01
```

The source CSV data is encoded with XOR `0x09`, except for bytes `0x0D`
and `0x0A`, which must be left unchanged. XORing those two bytes
destroys the original line endings and causes the decoded CSV to
collapse into a single line.

## Project scope

The server currently covers:

- launcher and authentication
- account and profile persistence
- inventory, store, equipment, and crafting
- channels, lobbies, and rooms
- clans
- friends and room invitations
- ProudNet game-session setup
- match lifecycle and result reporting

Some in-match state belongs to the LithTech listen host rather than
ProudNet. That creates a hard boundary for features that cannot be
controlled or repaired purely from this server.

See [PROGRESS.md](PROGRESS.md) for the exact state of each subsystem.

## Reverse-engineering notes

The implementation is based on protocol reconstruction, packet captures,
binary analysis, and runtime observation of the original client. Some
packet layouts are fully confirmed while others are only partially
understood.
 
## Disclaimer

This is an independent compatibility and preservation project. It is not
affiliated with or endorsed by the original game publisher or
developers.
