<img src="https://github.com/user-attachments/assets/edbee8de-18c1-4d0e-aaf4-a587e217a028" alt="MoleWire" width="420">

# MoleWire

Meet our tiny mole. He speaks the **[MoleWire protocol](docs/protocol.md)** a language he dug himself and tunnels files straight from one person to another over Tor. Safely, and anonymously.

**How it works**

1. The sender runs MoleWire and gets a temporary `.onion` address plus a SHA-256 hash.
2. Those two strings get handed to the receiver, however they like.
3. The receiver runs MoleWire, connects through Tor, and the file lands on their disk.

## Privacy

> **The shortest path between two people should not have to run through a third.**

* **There is no server in the middle at all.** The sender's machine is the server. Your file is never uploaded anywhere, never queued, and never sits on somebody else's disk. No accounts, no cloud storage, nobody's servers in between. One machine to the other, and that is the whole trip.
* **Neither side learns the other's IP address.** Both peers talk through Tor, so the transfer protocol never hands one side the other's normal address.
* **A fresh onion address is created for every transfer.** The onion service is discarded afterwards, so separate transfers do not share an onion identity.
* **Tor's own encryption does the work.** An onion-service connection is already encrypted end to end, and the `.onion` address is itself a public key, so reaching it proves you connected to the right machine. MoleWire deliberately does not bolt a home made crypto layer on top of that.

### What MoleWire does not promise

* **Not absolute anonymity.** MoleWire hides the peers' IP addresses **from each other**. It is **not a guarantee of anonymity** against every possible observer or against a compromised endpoint.
* **Tor cannot protect what your machine leaks.** Tor **cannot protect what the operating system, other applications, or the user deliberately expose**. A machine can still reveal identifying information through application behavior, logs, or other software running on it.
* **No metadata is stripped.** The file is sent exactly as it is. EXIF in photos, author fields in documents, and anything else baked into the file travels with it.
* **A matching hash does not mean a safe file.** The hash confirms that the received bytes match the expected file. It **does not mean the file is safe**, or that the sender is trustworthy.

## Usage

**You need**

* Go 1.25 or newer
* Tor installed, either as the `tor` daemon on `PATH` or as Tor Browser in its default location. MoleWire locates it automatically.

**Build**

```
git clone https://github.com/keyh4x/MoleWire.git
cd MoleWire
go build .
```

Builds for whatever machine it runs on. `MoleWire.exe` on Windows, `MoleWire` on Linux and macOS. One static binary, nothing to install alongside it.

**If Tor lives somewhere unusual**

```
export MOLEWIRE_TOR=/path/to/tor           # linux / mac
$env:MOLEWIRE_TOR = "C:\path\to\tor.exe"   # windows powershell
```

Applies to the current shell session only.

**Sending**

```
./MoleWire send secret.pdf
```

MoleWire starts Tor, publishes a fresh onion service and waits until the address is reachable. Then it prints:

```
Open to serve file now
File name :secret.pdf
File size :211 KB
File hash :6f2d6d55...bf9fe79e
Host link :mole2burrow.onion:52341
Share this :mole2burrow.onion:52341 6f2d6d55...bf9fe79e
```

The onion address and hash are shortened here to fit. Real ones are 56 and 64 characters.

The `Share this` line is everything the receiver needs. Keep the terminal open until the transfer finishes.

https://github.com/user-attachments/assets/3359fee6-a7a0-46a3-ad3c-ff9e0f016a1c

**Receiving**

```
./MoleWire get <onion:port> <hash> <download path>
```

Using the example above:

```
./MoleWire get mole2burrow.onion:52341 6f2d6d55...bf9fe79e /downloadpath
```

The hash is not optional. It is how the sender proves it is offering the file that was actually requested, and how the receiver confirms the bytes arrived intact.

If MoleWire reports that the burrow is not open yet, let it retry. A newly published onion address can take up to a minute before the rest of the Tor network is able to find it.

https://github.com/user-attachments/assets/4205f502-e3c7-47d0-931e-a223fa8e9bd0

## The MoleWire protocol

MoleWire speaks its own small binary protocol, built on six message types:

| message | purpose |
| --- | --- |
| `Burrow` | announces the protocol version and requests a file by hash |
| `Scent` | replies with the file name, size and chunk size |
| `Dig` | requests chunk N |
| `Dirt` | delivers chunk N |
| `Bury` | confirms the transfer is complete |
| `Collapse` | refuses the request and returns a reason code |

The six message layouts, how files are addressed by hash and how the request window works are in [docs/protocol.md](docs/protocol.md).

## License

MoleWire is licensed under the MIT License. For more details, refer to the [LICENSE](LICENSE) file.
