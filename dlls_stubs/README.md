# FEAR Online — DLL stubs

Two 32-bit Windows DLLs the client loads at startup:

- **`zlauncher_stub` → `ZLauncher.dll`** — a clean reimplementation of the
  launcher's network stub. It fakes a successful `LoginLauncher` and fires the
  connect/login callbacks so the launcher proceeds. Exports are pinned by
  `ZLauncher.def`.
- **`znetwork_stub` → `ZNetwork.dll`** — a logging proxy that forwards almost
  every export to the real `ZNetwork_orig.dll` (via linker pragmas in
  `forwards.h`) and intercepts a few for tracing.

Both are **x86 (32-bit)** and built with **MSVC** — the ZNetwork proxy uses
MSVC-only `/export` linker pragmas, so MinGW/Clang are not supported.

## Prerequisites

- Visual Studio 2022 (or Build Tools) with the C++ x86 toolchain
- CMake ≥ 3.21

## Build

From this folder:

```bat
cmake -G "Visual Studio 17 2022" -A Win32 -S . -B build
cmake --build build --config Release
```

`-A Win32` is required — a 64-bit DLL will not load into the 32-bit client. The
CMake config errors out early if the platform or compiler is wrong.

Output lands in:

```
build/dist/Release/ZLauncher.dll
build/dist/Release/ZNetwork.dll
```

## Deploy (into the game)

Back up the originals first.

**ZNetwork** (a proxy — needs the original alongside it) in
`FEAR_Online\Game\`:

1. Rename the shipped `ZNetwork.dll` → `ZNetwork_orig.dll`.
2. Copy the built `ZNetwork.dll` (proxy) into the same folder.

**ZLauncher** (a full replacement) in the game root:

1. Back up the shipped `ZLauncher.dll`.
2. Copy the built `ZLauncher.dll` over it.

To revert, restore the backups (and delete `ZNetwork_orig.dll`).

## Notes

- `forwards.h` is generated (`generate-forwards.py`) from the shipped
  `ZNetwork_orig.dll`'s export table. Regenerate it if that DLL changes; don't
  hand-edit.
- The pre-built `*.dll/.lib/.exp/.obj` next to the sources are stale artifacts
  from earlier manual builds and can be deleted — CMake produces fresh ones under
  `build/`.
