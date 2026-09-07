# Common::Standard::Clan::Member wire format

Recovered from `ZNetwork.dll`'s `FUN_100ca7e0` deserializer and verified against
`GameClient.dll`'s clan roster UI writer.

The native object is `0x8C` bytes. Its vtables, padding, string internals and
computed fields are not serialized.

| Wire order | Type | Native field | Client use |
|---:|---|---:|---|
| 1 | `u16` | `+0x24` | Stable member ID |
| 2 | Proud UTF-16 string | `+0x28` | Player name |
| 3 | `u8` | `+0x2C` | Player level |
| 4 | `6 × u32` | `+0x34..+0x48` | PvP: Win, Lose, Draw, Kill, Death, extra |
| 5 | `6 × u32` | `+0x54..+0x68` | Clan: Win, Lose, Draw, Kill, Death, extra |
| 6 | `u8` | `+0x70` | Channel index |
| 7 | `u8` | `+0x71` | Lobby index |
| 8 | `u16` | `+0x72` | Room index |
| 9 | `u8` | `+0x74` | Clan position (`1`, `2`, `8`, `9` are handled) |
| 10 | `u8` | `+0x78` | Logged in |
| 11 | `u8` | `+0x79` | In game |

Fixed bytes excluding the Proud string: **58 bytes**.

## Messages

- `0xA60F SendMemberList`: Proud varint count followed by member rows.
- `0xA610 NotifyMemberInfo`: one member row, without a count.
- `0xA611 NotifyMemberLevel`: `[u16 memberId][u8 level]`.
- `0xA612 NotifyMemberGameRecord`: member ID plus the PvP record object.
- `0xA613 NotifyMemberClanRecord`: member ID plus the clan record object.
- `0xA641 NotifyMemberPosition`: `[u16 memberId][i32 position]`.
- `0xA646/0xA647`: `[u16 memberId]`, login/logout.
- `0xA648`: `[u16 memberId][u8 channel]`.
- `0xA649`: `[u16 memberId][u8 lobby]`.
- `0xA64A`: `[u16 memberId][u16 room]`.
- `0xA64B/0xA64C`: `[u16 memberId]`, game start/end.

The sixth `u32` in each record is not exposed by the clan UI. The Go serializer
uses `Headshot` for the PvP extra and zero for the clan-record extra.
