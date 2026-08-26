# The MoleWire protocol

MoleWire is a pull protocol. Nothing gets pushed at you. The receiver asks for a chunk, the sender digs it out and sends it back, and that repeats until the file is done.

It runs over a normal TCP stream, which in practice is a Tor onion tunnel. There is no encryption of its own in here because Tor already does that part, and the `.onion` address is a public key, so if you reached it you reached the right machine.

## Messages

| byte | message | sent by | meaning |
| --- | --- | --- | --- |
| `0x01` | Burrow | receiver | handshake, requests a file by hash |
| `0x02` | Scent | sender | file metadata |
| `0x03` | Dig | receiver | requests chunk N |
| `0x04` | Dirt | sender | delivers chunk N |
| `0x05` | Bury | receiver | transfer complete |
| `0x06` | Collapse | sender | refusal, with a reason code |

### Burrow `0x01` (38 bytes)

| offset | size | field |
| --- | --- | --- |
| 0 | 1 | `0x01` |
| 1 | 4 | marker, the ASCII bytes `MOLE` |
| 5 | 1 | protocol version |
| 6 | 32 | SHA-256 of the requested file |

### Scent `0x02` (15 bytes plus the file name)

| offset | size | field |
| --- | --- | --- |
| 0 | 1 | `0x02` |
| 1 | 8 | file size in bytes, uint64 |
| 9 | 4 | chunk size in bytes, uint32 |
| 13 | 2 | name length, uint16 |
| 15 | that many | file name, UTF-8 |

The file name cannot be longer than 255 bytes.

### Dig `0x03` (5 bytes)

| offset | size | field |
| --- | --- | --- |
| 0 | 1 | `0x03` |
| 1 | 4 | chunk index, uint32, zero based |

### Dirt `0x04` (5 bytes plus the chunk)

| offset | size | field |
| --- | --- | --- |
| 0 | 1 | `0x04` |
| 1 | 4 | chunk index, uint32 |
| 5 | rest of the message | chunk bytes |

### Bury `0x05` (1 byte)

Just the type byte. No fields.

### Collapse `0x06` (2 bytes plus the reason)

| offset | size | field |
| --- | --- | --- |
| 0 | 1 | `0x06` |
| 1 | 1 | reason code |
| 2 | rest of the message | reason text, UTF-8 |

| code | meaning |
| --- | --- |
| `0x01` | unknown file |
| `0x02` | version mismatch |
| `0x03` | chunk index out of range |
| `0x04` | malformed message |
| `0x05` | internal error |

## How a transfer goes

Files are requested by SHA-256, never by name. The sender either holds that exact file or refuses. The receiver hashes what it wrote and compares before keeping anything.

```
receiver                          sender
   |                                 |
   |--- Burrow (version, hash) ----->|
   |<-- Scent (size, chunk, name) ---|
   |--- Dig 0 ---------------------->|
   |--- Dig 1 ---------------------->|   up to 128 outstanding
   |<-- Dirt 0 ----------------------|
   |--- Dig 128 -------------------->|   one new Dig per reply
   |<-- Dirt 2 ----------------------|   order is not guaranteed
   |<-- Dirt 1 ----------------------|
   |--- Bury ----------------------->|
```

The receiver does not ask for one chunk and then sit there waiting. It sends 128 requests at once, then sends one more every time a reply comes back, so there is always a pile of them in flight.

Due to Tor being slow one round trip (RTT) through the relays can take a few hundred milliseconds, so asking for chunks one at a time means doing nothing for most of the transfer. That caps your download at one 16 KB chunk per round trip. At 300 ms a trip that works out to about 53 KB/s, no matter how fast your internet is.

With 128 in flight you move 128 chunks per round trip instead of one, so the ceiling jumps to roughly 6.7 MB/s and what limits you goes back to being your actual Tor bandwidth.

## Constants

| name | value |
| --- | --- |
| marker | `MOLE` |
| max frame payload | 65536 bytes |
| default chunk size | 16384 bytes |
| max file name | 255 bytes |
| requests in flight | 128 |
