# FEAR Online Server - Progress

This document tracks implementation status, protocol findings,
unresolved behavior, and known server-side limitations.

Setup and client configuration belong in [README.md](README.md).

## Status definitions

| Status          | Meaning                                               |
|-----------------|-------------------------------------------------------|
| **Verified**    | Implemented and validated against the real client     |
| **Implemented** | Implemented, but not yet confirmed end-to-end in game |
| **Missing**     | Not implemented                                       |

## Launcher and authentication

| Feature                                         | Status       | Notes                                                                             |
|-------------------------------------------------|--------------|-----------------------------------------------------------------------------------|
| Steam SSO login (`/dialog/oauth/authorize`)     | **Verified** | Uses a session ticket rather than the manual login form                           |
| Patch / LivePatch (`/LivePatch/*`)              | **Verified** | Serves client files                                                               |
| ProudNet handshake + AES                        | **Verified** | Handles encrypted `0x26` and plaintext `0x01` envelopes                           |
| Two-connection credential (`0x7532` / `0x7533`) | **Verified** | Connection 1 issues the credential; connection 2 redeems it                       |
| Launcher marketing content                      | **Missing**  | `/fogame/notice`, `/news`, and `/contact` currently return empty placeholder HTML |

## Profile and progression

| Feature                                      | Status       | Notes                                                                                   |
|----------------------------------------------|--------------|-----------------------------------------------------------------------------------------|
| Profile login (`0x7595`)                     | **Verified** | Name, level, and EXP come from MongoDB                                                  |
| Point and Cash wallets (`0x762F` / `0x7693`) | **Verified** |                                                                                         |
| Records and PLAYER INFO (P vs P / P vs E)    | **Verified** | The client rebuilds the tabs from `SendMapQuestList 0x78E9`, not from aggregate records |
| Ranking / leaderboard                        | **Missing**  |                                                                                         |

## Inventory, store, and customization

| Feature                                                  | Status          | Notes                                                                                                                                                                                |
|----------------------------------------------------------|-----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Inventory and loadout on login                           | **Verified**    |                                                                                                                                                                                      |
| Buy item (`0x985B`)                                      | **Verified**    | Price and currency come from `GameItem.csv`                                                                                                                                          |
| Equip weapon / character (`0x985C`, `Item::EquipSlot`)   | **Verified**    |                                                                                                                                                                                      |
| Gear (`0x9863` / `0x9864`, `Item::Slot`)                 | **Verified**    | Mount / unmount                                                                                                                                                                      |
| Weapon custom parts (`0x9860` / `0x9861`, `Item::Equip`) | **Verified**    | Purchase mounts the part immediately; serial ordering is reversed relative to `Item::Slot`                                                                                           |
| Part / weapon compatibility (`PartLinkTable.csv`)        | **Implemented** | 175 links are loaded. Invalid combinations are rejected, and invalid purchases are blocked before charging the player                                                                |
| Custom-part stats (`CustomPartItem.csv`)                 | **Implemented** | Table is loaded but has no server-side consumer. `Item::Info` carries no attributes; the client calculates displayed stats from its own CSVs and combat damage belongs to the engine |
| Recipe crafting (`0x99EC`)                               | **Verified**    | Consumes materials from `Recipe.csv`                                                                                                                                                 |
| Rental expiration (24h / 7d / 30d)                       | **Verified**    | Each duration is a separate `itemIndex`; repurchasing extends the rental                                                                                                             |
| Count-based consumables                                  | **Verified**    | Stack count is stored in `value` and consumed by `FunctionIndex` through `SpendConsumable`; the client displays `1 Each`                                                             |
| Perks (`0x9AB4` / `0x9AB5`)                              | **Implemented** | Uses the gear path. `IncPerkSlot 0x9AB6` currently charges nothing and does not persist the unlocked slot count                                                                      |
| Craft error enum                                         | **Implemented** | Failures currently return code `0`, shown by the client as a generic error                                                                                                           |
| Use consumable (`0x985D` / `0x98C5`)                     | **Missing**     | Separate path from equipment; no consumable other than the emblem-related item is wired up                                                                                           |
| Psionics                                                 | **Missing**     |                                                                                                                                                                                      |
| External real-money / Cash purchase                      | **Missing**     |                                                                                                                                                                                      |

## Channels, lobby, and rooms

| Feature                                                          | Status          | Notes                                                                                                                                                       |
|------------------------------------------------------------------|-----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Channel list and join (`0x88B9` / `0x88BA`)                      | **Verified**    |                                                                                                                                                             |
| Enter / leave lobby (`0x8AAD` / `0x8AAE`)                        | **Verified**    |                                                                                                                                                             |
| Lobby, room, and user lists (`0x8AAF`-`0x8AB2`)                  | **Verified**    |                                                                                                                                                             |
| Create room (`0x8CA1`)                                           | **Verified**    | Handles title, password, and option block; room is published in a global registry                                                                           |
| Room list (`0x8AB0` / `0x8AB3`)                                  | **Implemented** | The `Room::Info` tail is currently interpreted as `[current][max][type]` followed by `public, state, teamBal, instruction, deathChat, perkUse, thirdView`   |
| Join room (`0x8CA2`)                                             | **Verified**    | `[u16 roomNumber][ProudString password][u16]` confirmed from capture; assigns a unique seat and balances teams                                              |
| Leave room (`0x8CA3`)                                            | **Verified**    |                                                                                                                                                             |
| Ready (`0x8CA6`)                                                 | **Implemented** | Tracked per player. Ready state was moved to the third byte of `Room::User`; it was previously written into the team field                                  |
| Available maps (`MapInfo.csv`)                                   | **Verified**    |                                                                                                                                                             |
| Change team / kick player / modify room options                  | **Missing**     |                                                                                                                                                             |
| General, clan, and whisper chat (`0x9471` / `0x9472` / `0x9473`) | **Implemented** | Request is `[text][u8 flag]`; broadcast is `0x94D5 [sender][text][u8 type][u8 flag]`                                                                        |
| Server notice (`0x9474`)                                         | **Missing**     | `chatType 6` is the GM / notice style                                                                                                                       |
| Matchmaking / quick join                                         | **Missing**     |                                                                                                                                                             |
| Match start                                                      | **Verified**    | Playable end-to-end. The START button does not emit `0x8CA7`; current workaround starts the match when the room leader becomes Ready (`solo_start_hack.go`) |

## Clans

| Feature                                        | Status          | Notes                                                                                                                                                               |
|------------------------------------------------|-----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Check clan name (`0xA416`)                     | **Verified**    |                                                                                                                                                                     |
| Create clan (`0xA417`)                         | **Verified**    | Level gate is client-side                                                                                                                                           |
| Search clan (`0xA414`)                         | **Verified**    |                                                                                                                                                                     |
| Clan panel: `Clan::Info`, roster, counters     | **Verified**    | `0xA60B` / `0xA60F` / `0xA642` / `0xA074`                                                                                                                           |
| Applicant queue (`0xA617`, `Clan::Request`)    | **Verified**    | Applicant appears in the master’s table without relogging                                                                                                           |
| Accept applicant (`0xA420`)                    | **Verified**    | Adds to roster through `0xA610` + `0xA642` and removes from queue through `0xA618`                                                                                  |
| Reject applicant (`0xA421`)                    | **Verified**    | Removes the applicant from the master’s queue                                                                                                                       |
| Kick member (`0xA422`)                         | **Verified**    | A manager cannot kick another manager; nobody can kick the master                                                                                                   |
| Member departure (`0xA614`)                    | **Verified**    | Remaining members receive an immediate roster update                                                                                                                |
| Leave clan (`0xA423`)                          | **Verified**    | Master is blocked while other members remain                                                                                                                        |
| Change rank (`0xA425`)                         | **Verified**    | Only the master can assign ranks; `0xA641` updates the roster immediately                                                                                           |
| Request to join (`0xA41E`) / cancel (`0xA41F`) | **Implemented** | Service logic is tested, but the RMI handler has not yet been triggered by a real client                                                                            |
| Transfer leadership (`0xA424`)                 | **Implemented** | Departing master becomes a manager                                                                                                                                  |
| Delete clan (`0xA418`)                         | **Verified**    | Master only; detaches the roster and clears the clan UI for online members using `0xA615` + `0xA60C`                                                                |
| Validate emblem (`0xA429`)                     | **Verified**    | Response carries the verdict in a byte rather than in the response ID                                                                                               |
| Save emblem (`0xA42A`)                         | **Verified**    | Persists, broadcasts `0xA657`, and consumes a `Clan Mark Change` (`FunctionIndex 1901`, free or paid). Reapplying the current emblem is a no-op and does not charge |
| Member presence (channel / lobby / room)       | **Missing**     | Events `0xA646`-`0xA64C`; roster snapshot does not yet carry room state                                                                                             |
| Clan chat (`0xA427`)                           | **Verified**    | Broadcasts `0xA64F [sender][text]` to all clan members, including the sender                                                                                        |
| Rename clan / clan records                     | **Missing**     | `0xA419`, `0xA428`                                                                                                                                                  |
| Rank values `8=Member` / `9=Associate`         | **Implemented** | `1=Master` and `2=Manager` are confirmed in game; the other mappings remain inferred                                                                                |

## Match

| Feature                                                             | Status          | Notes                                                                                        |
|---------------------------------------------------------------------|-----------------|----------------------------------------------------------------------------------------------|
| Game connection (`0x9025` / `0x9089`)                               | **Verified**    | Second ProudNet connection; ticket travels in the two GUID bytes that survive the round trip |
| `0x9057 NotifyGameStart`                                            | **Verified**    | `[host][u16 port][Game::UserKey]`, points to this server                                     |
| `0x921C NotifyHostServer`                                           | **Verified**    | `[host][u16 port][u8 isHost][u8 bIntrude]`; this is the LithTech address, not the map name   |
| Match snapshot (`0x9219`-`0x9225`)                                  | **Verified**    | Map, match info, items, player list, and equipment                                           |
| Load progress (`0x908B` / `0x908C`)                                 | **Verified**    | Acknowledged and broadcast; match starts only after all players enter                        |
| Round start (`0x908D` / `0x908E`)                                   | **Verified**    |                                                                                              |
| Death (`0x9093`)                                                    | **Verified**    | `[killer][victim][assist][u32][3 x u8]`; kills, deaths, and assists are counted              |
| Respawn (`0x908F` / `0x9090` / `0x9091`)                            | **Verified**    |                                                                                              |
| Round and game finish (`0x9092` -\> `0x9221` / `0x922B` / `0x922C`) | **Verified**    | Results screen is displayed                                                                  |
| Session ping (`0x909C`) and hit count (`0x9028`)                    | **Verified**    | Both only require acknowledgement                                                            |
| In-match equipment (`0x909D` / `0x9223` / `0x9224`)                 | **Verified**    | `0x909D` is a query; server returns the player’s equipment set                               |
| In-match weapon change (`0x909A`)                                   | **Implemented** | Received, validated, and persisted, but does not affect respawn during the current match     |
| End-of-match scoreboard (`0x9221`)                                  | **Implemented** | Kill / death / assist data is sent. EXP and GP come from `0x9222`, which is not implemented  |
| End-of-match reward (`0x9222`)                                      | **Missing**     | `Game::UserReward = [u8][u16][u16][u32]`; the item field has not been identified             |
| In-game Tab scoreboard                                              | **Missing**     | Not reachable through this server; see Known limitations                                     |

## Social

| Feature                                                   | Status          | Notes                                                                                                         |
|-----------------------------------------------------------|-----------------|---------------------------------------------------------------------------------------------------------------|
| Session registry by SteamID                               | **Verified**    | Base for player-to-player push events                                                                         |
| Add / accept / reject / remove friend (`0xA02A`-`0xA02D`) | **Verified**    | Mutual relationship on both accounts; login lists use `0xA05D` / `0xA05E`, and online requests use `0xA061`   |
| Invite friend to room (`0xA02E`)                          | **Verified**    | `Room::Invite` renders room number, title, mode, map, and objective; accepting joins the room                 |
| Friend presence (`0xA06D`-`0xA073`)                       | **Implemented** | Snapshot already carries online and room state; presence deltas and channel / lobby state are not emitted yet |
| Private message / mail / gift                             | **Missing**     |                                                                                                               |

## Rewards, long-term progression, and events

The catalogs are already decoded and present under
`internal/catalog/data`; what is missing here is server behavior rather
than source data.

Eight of the 18 CSV catalogs are currently not referenced by code:
`CapsuleItem`, `CapsuleReward`, `Curse`, `EtcInfo`, `EventAttendance`,
`Mission`, `Nickname`, and `PveAiInfo`.

| Feature                                                  | Status          | Notes                                                                                                                                                                                                                        |
|----------------------------------------------------------|-----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| End-of-match reward (`RewardItem.csv`)                   | **Missing**     | XP, Point, and item use RNG ranges. Index comes from `WinRewardItem`, `LoseRewardIndex`, or `DrawRewardIndex` in `MapInfo.csv`. This is what should populate EXP / GP / Item on the results screen, which are currently zero |
| Level-up (`ClassInfo.csv`)                               | **Missing**     | Curve is already read to resolve level; level-up rewards are not                                                                                                                                                             |
| Capsule / gacha (`CapsuleItem.csv`, `CapsuleReward.csv`) | **Missing**     | Needs cost handling, auditable RNG, atomic consume / grant, and grade announcement                                                                                                                                           |
| Missions (`Mission.csv`)                                 | **Missing**     | Conditions by mode / map / difficulty, `ActionType` / `ActionCount`, and up to three reward items                                                                                                                            |
| Challenges (`Challenges.csv`)                            | **Missing**     | Four milestones per challenge, each with EXP, Point, item, and medal; each milestone should be claimable once                                                                                                                |
| Event Attendance (`EventAttendance.csv`)                 | **Missing**     | One claim per server-defined period                                                                                                                                                                                          |
| MapQuest / WeaponQuest                                   | **Implemented** | `SendMapQuestList 0x78E9` is sent and is what the client uses to build PLAYER INFO tabs; progress itself is not accumulated                                                                                                  |
| Medals                                                   | **Missing**     | Challenge milestone reward                                                                                                                                                                                                   |

## PvE

| Feature                           | Status      | Notes                                                                                                                                             |
|-----------------------------------|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| PvE mode                          | **Missing** | Six maps in `MapInfo.csv` (2 missions x 3 difficulties), marked `PlayPve=1` and `PlayNormal=0`                                                    |
| Ticket access                     | **Missing** | Five `PVE_Ticket` entries exist in `GameItem.csv`, `CashType=1`, from 0 to 1399. PvE appears to be ticket-gated content rather than a season pass |
| PvE-only items                    | **Missing** | `OnlyPvE` in `GameItem.csv` restricts use to the mode                                                                                             |
| AI contribution (`PveAiInfo.csv`) | **Missing** | `AiKillConPoint` per enemy type                                                                                                                   |
| Groggy, revive, and assist        | **Missing** | `UsePlayerGroggy`, `PlayerGroggyTime`, and `UsePveZombie` exist in the engine registry                                                            |
| PvE rank (`0x9029`)               | **Missing** | RMI exists but is not handled                                                                                                                     |

## Content rules and moderation

| Feature                           | Status          | Notes                                                                                                                    |
|-----------------------------------|-----------------|--------------------------------------------------------------------------------------------------------------------------|
| Buy / equip level limits          | **Missing**     | `BuyLevelLimit` and `EquipLevelLimit` exist in `GameItem.csv` but are not checked                                        |
| Ranked matches                    | **Missing**     | `MatchGame=1` appears on three Demolition maps; no rank gate or matchmaking exists                                       |
| Profanity filter (`Curse.csv`)    | **Missing**     | Catalog decoded but unused                                                                                               |
| Nickname rules (`Nickname.csv`)   | **Missing**     | Catalog decoded but unused                                                                                               |
| Item actions (`FunctionItem.csv`) | **Implemented** | Only consumable `FunctionIndex` handling is wired up                                                                     |
| Report / mute / ban               | **Missing**     |                                                                                                                          |
| Chat rate limiting / anti-spam    | **Missing**     |                                                                                                                          |
| Spectator / voting                | **Missing**     | Present in the engine registry, but no RMI has been mapped                                                               |
| Match integrity                   | **Missing**     | Server trusts the listen host. There is no `matchId`, event sequence, or audit log for store / craft / reward operations |

## Known limitations

Most unresolved in-match behavior reaches the same architectural
boundary: ProudNet owns account and room authority, while live match
state belongs to the LithTech engine and listen host.

### Not reachable from the server

| Area                              | What is known                                                                                                                                                                                                                                                                                                                                                                                                 |
|-----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| In-game Tab scoreboard            | The panel opens but player rows are not drawn. Runtime observation shows the publishing function at `0x10092660` is called, both guards pass, the engine player list contains the correct node, and the Scaleform target is valid. Data reaches ActionScript, but ActionScript does not draw it. No ProudNet S2C path reaches this behavior. See `INMATCH_TAB_MAP.md` and `PEDIDO_TAB_IN_MATCH.md`            |
| `Game::UserEquip` is dead storage | ZNetwork stores the snapshot at `HostData+0x388` and never reads it again. `GetServerHostData` is loaded by GameClient and GameServer but never called, and `HOST_GAME_USER_EQUIP` is never registered. The `u32` domain therefore cannot be confirmed as ItemIndex vs serial because there is no consumer. The server sends ItemIndex because it is the only identity with the same meaning on both machines |
| Weapon change on respawn          | `0x909A` is received and persisted, but there is no path to notify the current client. The five `HOST_GAME_*` setters for weapon change, user equip, PvE rank, user item, and team change are loaded but never invoked to register handlers. The change takes effect in the next match                                                                                                                        |
| START button                      | It never emits `0x8CA7`, even with a full room and every player ready. The same behavior was observed in the earlier Node proof of concept. The condition that arms the button has not been found. Current workaround starts the match from the leader’s Ready state                                                                                                                                          |

### Wire layout known, meaning still unknown

| Area                                                             | Unknowns                                                                                                                                                                                                                          |
|------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `0x9093 RequestDeath`                                            | First three bytes are seats for killer, victim, and assist. The `u32` at +3 and final three bytes remain unknown; the only capture had them all set to zero                                                                       |
| `0x9028 RequestHitCount`                                         | Contains five `u16` values and receives an empty response. No names for the counters were found in the binary                                                                                                                     |
| `Game::UserReward`                                               | Layout is `[u8 seat][u16][u16][u32]`. The final three values reach ActionScript as arguments 16, 17, and 18 of `ChangePlayerInfo_Sh`; which one is the item is decided inside compiled ActionScript where names are not preserved |
| `Game::UserResult` counter order                                 | Wire positions and ActionScript argument order are confirmed. Interpreting the four values as kill, death, assist, and score remains an inference                                                                                 |
| Tail of `Game::UserResult` and extra fields in `Game::UserEquip` | Final seven bytes of `Game::UserResult` and five extra `u32` fields in `Game::UserEquip` are currently sent as zero. Filling them from coincidental counts would be guesswork                                                     |
| Clan `profileCharacterID` as `u16`                               | `NotifyUserClanInfo` receives identity truncated to 16 bits. It is probably a different identifier, such as a member ID, rather than the account ID                                                                               |

### Not yet tested

| Area                           | Reason                                                                                                                                                                                            |
|--------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Two-player match               | Required account identity work is complete, the ZNetwork proxy has been removed, and launcher behavior is fixed. Remaining step is to point `LOGIN_SERVER_IP` at the LAN address and run the test |
| Multiple rounds per match      | `FinishRound` advances until `RoundLimit`, but only single-round matches have been played                                                                                                         |
| Perks in game                  | Persisted and served, but their gameplay effect has not been observed                                                                                                                             |
| Chat during a match            | Implemented in the lobby; unknown whether the client changes channel while in game                                                                                                                |
| Friend presence during a match | Snapshot includes room and online state, but `0xA06D`-`0xA073` deltas are not emitted                                                                                                             |
