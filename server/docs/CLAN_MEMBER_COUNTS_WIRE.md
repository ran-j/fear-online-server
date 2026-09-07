# Common::Standard::Clan aggregate member counts

The roster vector and the clan summary counters are independent client state.

## `NotifyMemberCount` — `0xA642`

```text
Offset  Type  Meaning
0x00    u8    current member total, including the master
0x01    u8    manager count (`position == 2`)
0x02    u8    member count (`position == 8`)
0x03    u8    associate count (`position == 9`)
0x04    u8    member capacity
```

Client-side destination fields:

```text
manager + 0x1ED0 = current total
manager + 0x1ED4 = managers
manager + 0x1ED8 = members
manager + 0x1EDC = associates
manager + 0x1EE0 = capacity
```

`SendMemberList (0xA60F)` builds the member rows but does not initialize these
five fields. `NotifyMemberInfo (0xA610)` and delete-member events can update the
current total from vector size, but login still requires `Clan::Info` counts or
`0xA642`.

## Clan positions

```text
1 = Master
2 = Manager
8 = Member
9 = Associate
```

## `Clan::Info` correction

After the four-u16 clan mark, the serializer carries:

```text
[u8 unknown presentation field]
[u8 current total]
[u8 manager count]
[u8 member count]
[u8 associate count]
[u8 member capacity]
```

The two bytes before the u64 clan experience are:

```text
[u8 unknown]
[u8 clan level]
```
