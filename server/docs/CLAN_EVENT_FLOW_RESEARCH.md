# FEAR Online — Clan event flow and online-session architecture

This report compares the current Go server in `server(3).zip` with the clan protocol recovered from `ZNetwork.dll`, `GameClient.dll`, and the RMI map.

## 1. Main conclusion

The clan protocol is not request/response-only. It has three distinct layers:

1. **Command acknowledgment** sent to the session that made the request (`Answer*`).
2. **Clan-state deltas** sent to every affected clan UI (`Send*` / `Notify*`).
3. **Global identity and presence updates** sent to sessions that need to display the player outside the clan page (`NotifyUserClanInfo`, login/logout/location notifications).

`AnswerAcceptMemberSuccess (0xA635)` only acknowledges the master's command. It cannot make the accepted player's client join the clan by itself.

The current Go server has no `SteamID -> live sessions` registry, so it cannot implement layers 2 and 3 for another player. This is a real architectural gap, not a one-off clan bug.

## 2. Current-server gaps

### 2.1 No cross-session addressability

`Credentials` maps a temporary token to a `models.Player`, but the server has no way to find the live `Session` for a SteamID after authentication.

### 2.2 Concurrent pushes would corrupt transport state

`Session.SendPlain`, `Session.SendEncrypted`, handshake writes, and heartbeat writes all write directly to the same `net.Conn`. Once clan/friend/room events are sent asynchronously, writes can interleave. `encryptCount` is also unprotected.

A per-session send mutex is required before adding push events.

### 2.3 Session player snapshots become stale

`sessionData` stores a complete `models.Player` value. If a clan master accepts another online player, the accepted player's database row changes from another session, but that player's cached session copy still has the old `ClanName`.

Long term, session state should hold stable identity, not be the canonical mutable player aggregate. At minimum, every domain mutation must refresh all affected session snapshots.

### 2.4 Missing protocol IDs in the data model

The binary identifies members and applicants with **16-bit IDs**, not SteamIDs and not array indexes.

The current models lack:

- `ClanMember.MemberID uint16`
- `ClanApplicant.ApplicantID uint16`

These IDs are required by accept, reject, kick, role, presence, location, and game-state deltas.

### 2.5 Clan roles do not match the wire values

The client handles role values `1`, `2`, `8`, and `9`. The C2S wrapper for changing a role only permits `2`, `8`, and `9`; incoming notifications also accept `1`, which is strongly consistent with the master role.

The current clan creator stores `Rank: 0`, which is not one of the client-handled values.

Confirmed values:

- `1`: accepted by the client, likely master.
- `2`, `8`, `9`: accepted by both client state and the C2S change-position wrapper.

The names of `2`, `8`, and `9` are not yet proven from the binary, so they should be represented as protocol constants without invented labels until the UI strings are mapped.

## 3. C2S clan commands

| ID | Name | Confirmed request body |
|---|---|---|
| `0xA411` | `RequestLogin` | not fully mapped |
| `0xA412` | `RequestOpen` | empty |
| `0xA413` | `RequestClose` | empty |
| `0xA414` | `RequestSearchName` | Proud string |
| `0xA415` | `RequestSearchMaster` | Proud string |
| `0xA416` | `RequestCheckName` | Proud string |
| `0xA417` | `RequestCreate` | clan name + four `u16` mark values at the typed API level |
| `0xA418` | `RequestDelete` | empty |
| `0xA419` | `RequestChangeClanName` | Proud string |
| `0xA41A` | `RequestUpdateNotice` | Proud string |
| `0xA41B` | `RequestUpdateIntro` | Proud string |
| `0xA41C` | `RequestUpdateClanRecord` | record payload, not fully mapped |
| `0xA41D` | `RequestClearClanRecord` | empty or no meaningful argument |
| `0xA41E` | `RequestJoin` | Proud string clan name |
| `0xA41F` | `RequestJoinCancel` | empty |
| `0xA420` | `RequestAcceptMember` | `[u16 applicantId]` |
| `0xA421` | `RequestRejectMember` | `[u16 applicantId]` |
| `0xA422` | `RequestKickOutMember` | `[u16 memberId]` |
| `0xA423` | `RequestGetOut` | empty |
| `0xA424` | `RequestChangeMaster` | `[u16 memberId]` |
| `0xA425` | `RequestChangePosition` | `[u16 memberId][i32 position]` |
| `0xA426` | `RequestInviteRoom` | `[u16 userId]` |
| `0xA427` | `RequestClanChat` | chat payload, likely string; exact body still to verify |
| `0xA428` | `RequestLoginClanRecord` | one byte in the generated sender path |
| `0xA429` | `RequestCheckMark` | four `u16` values |
| `0xA42A` | `RequestChangeMark` | four `u16` values |

## 4. S2C state model recovered from the client

### 4.1 Clan aggregate

- `0xA60B SendClanInfo` carries one full `Common::Standard::Clan::Info`.
- Client handler: `FUN_100564E0` -> copies the full object with `FUN_10057F60`.
- It also clears the local pending-request state and invokes the clan-info refresh callback.

`0xA60C SendClearClanInfo` clears clan/member/request state and refreshes the UI.

`0xA60D SendRequestClanInfo` is the applicant's **own pending application summary**. It is separate from the master's request list.

`0xA60E SendClearRequestInfo` clears that pending-application summary.

### 4.2 Member roster

| ID | Client effect | Confirmed payload shape |
|---|---|---|
| `0xA60F SendMemberList` | Clears/builds the roster vector | list of `Clan::Member` |
| `0xA610 NotifyMemberInfo` | Inserts one member and recalculates member count | one `Clan::Member` |
| `0xA611 NotifyMemberLevel` | Changes level field | `[u16 memberId][u8 level]` |
| `0xA612 NotifyMemberPvPRecord` | Updates PvP record | `PvPRecord` object containing member identity |
| `0xA613 NotifyMemberClanRecord` | Updates clan record | `Clan::Record` object containing member identity |
| `0xA614 NotifyDeleteMember` | Removes one member and recalculates count | `[u16 memberId]` |
| `0xA615 SendClearMember` | Clears entire member vector | empty |
| `0xA641 NotifyMemberPosition` | Writes role at member `+0xB0` | `[u16 memberId][i32 position]` |
| `0xA642 NotifyMemberCount` | Writes five count/status fields | `[u8][u8][u8][u8][u8]` |

`NotifyMemberCount` is **not** one integer. Its handler writes the five bytes to client state offsets `+0x1ED0`, `+0x1ED4`, `+0x1ED8`, `+0x1EDC`, and `+0x1EE0`. The first field is demonstrably the displayed/current member count. The exact semantics of the remaining four fields are not yet proven and should not be named speculatively.

### 4.3 Membership requests

| ID | Client effect | Confirmed payload shape |
|---|---|---|
| `0xA616 SendRequestList` | Builds master's applicant table | list of `Clan::Request` |
| `0xA617 NotifyRequestInfo` | Inserts one applicant | one `Clan::Request` |
| `0xA618 NotifyDeleteRequest` | Removes one applicant | `[u16 applicantId]` |
| `0xA619 SendClearRequest` | Clears master's applicant table | empty |

`Clan::Request` is a real structured object, not just a name. Its recovered native object includes applicant ID, strings, level, PvP record, and application date. The current `ClanApplicant` model contains most business information but still lacks the stable `u16 ApplicantID` needed by the wire protocol.

### 4.4 Presence and location

| ID | Effect | Confirmed payload |
|---|---|---|
| `0xA646 NotifyLogIn` | member online flag = 1 | `[u16 memberId]` |
| `0xA647 NotifyLogOut` | member online flag = 0 | `[u16 memberId]` |
| `0xA648 NotifyJoinChannel` | updates member channel state | `[u16 memberId][u8 channel]` |
| `0xA649 NotifyJoinLobby` | updates member lobby state | `[u16 memberId][u8 lobby]` |
| `0xA64A NotifyJoinRoom` | updates room state | `[u16 memberId][u16 roomId]` |
| `0xA64B NotifyGameStart` | in-game flag = 1 | `[u16 memberId]` |
| `0xA64C NotifyGameEnd` | in-game flag = 0 | `[u16 memberId]` |

These are clan-presence events. They are distinct from similarly named friend notifications.

### 4.5 Chat and room invitation

- `0xA645 SendInviteRoom` carries the target/inviter ID plus a room-information object.
- `0xA64F NotifyClanChat` invokes the UI with **two Proud strings**. They are most likely sender/name and message, but their exact semantic order still requires a live capture or sender serializer trace.

## 5. Correct event flows

The following packet order is partly protocol-verified and partly inferred from the state mutations. Inferred ordering is marked explicitly.

### 5.1 Player requests to join

Database/domain operation:

1. Validate player is not already in a clan.
2. Allocate or reuse stable `ApplicantID`.
3. Insert application idempotently.

Applicant session:

1. `0xA631 AnswerJoinSuccess`.
2. `0xA60D SendRequestClanInfo` so the waiting-for-approval panel survives navigation/relogin.

Online master/officers:

1. `0xA617 NotifyRequestInfo` with the new `Clan::Request`.
2. Possibly `0xA642 NotifyMemberCount` if one of its unknown counters is pending-request count. This is plausible but not yet proven.

### 5.2 Master accepts applicant

#### Atomic domain mutation

The accept command must do all of this as one logical transaction:

1. Validate actor belongs to clan and has permission.
2. Find application by `ApplicantID`.
3. Recheck applicant still has no clan.
4. Remove applicant from pending list.
5. Allocate stable `MemberID`.
6. Add member with a valid protocol role.
7. Set applicant player's `ClanName`.
8. Persist clan and player atomically, ideally with an outbox event.

#### Master/requesting session

1. `0xA635 AnswerAcceptMemberSuccess` — command acknowledgment only.
2. `0xA618 NotifyDeleteRequest [applicantId]` — removes the row from the request table.
3. `0xA610 NotifyMemberInfo [Clan::Member]` — inserts the accepted member in the roster.
4. `0xA642 NotifyMemberCount [5 bytes]` — refreshes aggregate counters.

#### Other online clan members

1. `0xA610 NotifyMemberInfo`.
2. `0xA642 NotifyMemberCount`.
3. `0xA646 NotifyLogIn [memberId]` if the applicant currently has an online session and the `Clan::Member` snapshot did not already mark them online.

#### Accepted player's online sessions

1. `0xA60E SendClearRequestInfo` — clear the applicant's waiting state.
2. `0xA60B SendClanInfo` — install full clan aggregate.
3. `0xA60F SendMemberList` — install full roster.
4. `0xA074 NotifyUserClanInfo` — update clan tag/emblem in Player Info and global social UI.

The exact original ordering between `A60B`, `A60F`, and `A60E` is not proven, but `A60B` itself clears pending-request state in the client, so sending `A60E` is safe but may be redundant. `A60B` must precede or accompany the roster bootstrap.

### 5.3 Reject applicant

Master:

1. `0xA637 AnswerRejectMemberSuccess`.
2. `0xA618 NotifyDeleteRequest [applicantId]`.

Applicant online:

1. `0xA60E SendClearRequestInfo`.

No clan tag/member state is installed.

### 5.4 Applicant cancels request

Applicant:

1. `0xA633 AnswerJoinCancelSuccess`.
2. `0xA60E SendClearRequestInfo`.

Master/officers online:

1. `0xA618 NotifyDeleteRequest [applicantId]`.

### 5.5 Kick member

Requester/master:

1. `0xA639 AnswerKickOutMemberSuccess`.

Remaining clan members:

1. `0xA614 NotifyDeleteMember [memberId]`.
2. `0xA642 NotifyMemberCount`.

Kicked player's online sessions:

1. `0xA60C SendClearClanInfo`.
2. `0xA615 SendClearMember` if not already covered by `A60C`.
3. `0xA074 NotifyUserClanInfo` with empty clan name and zero mark.

Database:

- Clear player's `ClanName` in the same transaction as member removal.

### 5.6 Voluntary leave

Leaving player:

1. `0xA63B AnswerGetOutSuccess`.
2. Same clear sequence as a kicked member.

Remaining members:

1. `0xA614 NotifyDeleteMember`.
2. `0xA642 NotifyMemberCount`.

The master should not be allowed to leave without transferring master or deleting the clan unless the original business rule proves otherwise.

### 5.7 Change role

Requester:

1. `0xA63F AnswerChangePositionSuccess`.

All online clan sessions:

1. `0xA641 NotifyMemberPosition [memberId, role]`.

### 5.8 Change master

Requester:

1. `0xA63D AnswerChangeMasterSuccess`.

All online clan sessions need at least:

1. `0xA641` for the old master's new role.
2. `0xA641` for the new master's role `1`.
3. A refreshed `0xA60B SendClanInfo`, because master identity/name is part of the clan aggregate and there is no dedicated `NotifyMaster` RMI.

This fanout is a strong protocol inference; the exact original packet order was not recovered.

### 5.9 Login and disconnect presence

After a clan member's session becomes authenticated:

1. Load current clan and locate member by SteamID.
2. Register session in online registry.
3. Broadcast `0xA646 NotifyLogIn [memberId]` to other online clan sessions when the player's first live session appears.
4. Send a presence snapshot/full `MemberList` to the new session.

On disconnect:

1. Remove session from registry.
2. Only broadcast `0xA647 NotifyLogOut` when the player's **last** live session disappears.

This first/last-session rule prevents false logout events during login-to-lobby connection transitions or multiple connections.

## 6. Recommended server architecture

### 6.1 Session registry

Required index:

```go
SteamID -> set of *Session
Session -> SteamID
```

A set is necessary because one account can temporarily have multiple ProudNet connections.

Required operations:

- `Bind(steamID, session)`
- `Unbind(session)`
- `Sessions(steamID) []Session`
- `IsOnline(steamID) bool`
- later: `SessionsForClan(clanName)` or fanout based on clan member SteamIDs

Never hold the registry lock while writing to a socket.

### 6.2 Serialized socket writes

Every path that writes to a `Session` must share one mutex:

- plain RMIs
- encrypted RMIs and `encryptCount`
- handshake/control frames
- heartbeat replies
- asynchronous domain-event pushes

Without this, adding a registry creates an intermittent frame-corruption bug.

### 6.3 Domain command + event fanout

Do not put cross-player side effects directly into the repository.

Recommended shape:

```go
type ClanMemberAccepted struct {
    Clan       models.Clan
    Applicant  models.ClanApplicant
    Member     models.ClanMember
    ActorSteamID string
}
```

`ClanService.AcceptMember` performs the transaction and returns a result/domain event. The ProudNet clan event dispatcher translates that result into RMIs for:

- requester
- accepted player
- other online members

The same registry/dispatcher can later serve friends, room invitations, clan chat, party systems, and presence.

### 6.4 Direct database edits

A raw Mongo edit cannot automatically reproduce the semantic event sequence.

Preferred options, in order:

1. Admin panel/API calls the same `ClanService.AcceptMember` command.
2. Transactional outbox written with the clan/player mutation, consumed by an event worker.
3. Mongo change streams as a compatibility bridge for manual edits.

Change streams alone are weaker because a document diff does not reliably identify actor, intent, old role, applicant ID, or which packets have already been sent. If used, events need a clan revision/idempotency key.

## 7. Concrete schema changes before implementing accept

```go
type ClanMember struct {
    MemberID uint16
    SteamID  string
    Name     string
    Rank     uint8
    // player level, PvP record, clan record, presence fields...
}

type ClanApplicant struct {
    ApplicantID uint16
    SteamID     string
    Name        string
    Level       uint32
    Message     string
    AppliedAt   time.Time
}
```

Also add a stable clan numeric ID instead of deriving it from CRC32 once requests and selections must reference the same clan across renames.

ID allocation must avoid zero and handle the `uint16` range. It should be persisted, never derived from array position.

## 8. Safe implementation order

1. Add session send lock and online registry.
2. Add stable `MemberID` / `ApplicantID` and migrate existing documents.
3. Map and implement the complete `Clan::Member` serializer.
4. Implement `RequestOpen/Login` bootstrap: `A60B`, `A60F`, `A616` or `A60D` depending role/state.
5. Implement `RequestAcceptMember` with atomic mutation and fanout.
6. Add reject/cancel/kick/leave and role/master changes.
7. Add presence events and clan chat.
8. Add outbox/admin command path so external approval also produces pushes.

Do not implement `A420` by only changing Mongo and sending `A635`; that would preserve the exact architecture bug being fixed.
