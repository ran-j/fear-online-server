// ZNetwork.dll proxy with logging.

// Objective: the client stops before opening any socket when a match
// starts, and the decision happens inside Client_On_GAME_START_Callback
// (GameClient.dll FUN_10151060), which picks between a local path and dialling
// a "Dedi IP : Port". Static analysis cannot say which branch runs, so this
// observes it.

#include <windows.h>
#include <stdio.h>
#include <stdarg.h>

#include "forwards.h"

#define ORIG_DLL_NAME L"ZNetwork_orig.dll"
#define LOG_FILE_NAME L"znetwork_proxy.log"
#define CONSOLE_LOG_NAME L"client_console.log"
#define CAPTURE_SENTINEL_NAME L"znproxy_capture.on"

static HMODULE g_orig;
static CRITICAL_SECTION g_log_lock;
static int g_log_ready;
static WCHAR g_log_path[MAX_PATH];
static WCHAR g_console_log_path[MAX_PATH];
static WCHAR g_sentinel_path[MAX_PATH];

static void capture_client_stdout(void)
{
    FILE *redirected = NULL;

    if (g_console_log_path[0] == 0)
    {
        return;
    }

    if (GetConsoleWindow() == NULL)
    {
        AllocConsole();
    }

    _wfreopen_s(&redirected, g_console_log_path, L"w", stdout);
    _wfreopen_s(&redirected, g_console_log_path, L"a", stderr);
    setvbuf(stdout, NULL, _IONBF, 0);
    setvbuf(stderr, NULL, _IONBF, 0);
    fputs("---- client stdout capture start ----\n", stdout);
    fflush(stdout);
}

// ---------------------------------------------------------------- logging

static void log_line(const char *fmt, ...)
{
    va_list args;
    FILE *fh;

    if (!g_log_ready)
    {
        return;
    }

    EnterCriticalSection(&g_log_lock);
    if (_wfopen_s(&fh, g_log_path, L"a") == 0 && fh)
    {
        SYSTEMTIME now;
        GetLocalTime(&now);
        fprintf(fh, "[%02u:%02u:%02u.%03u] ",
                now.wHour, now.wMinute, now.wSecond, now.wMilliseconds);
        va_start(args, fmt);
        vfprintf(fh, fmt, args);
        va_end(args);
        fputc('\n', fh);
        fclose(fh);
    }
    LeaveCriticalSection(&g_log_lock);
}

static int safe_read(const void *src, void *dst, SIZE_T len)
{
    SIZE_T copied = 0;
    return ReadProcessMemory(GetCurrentProcess(), src, dst, len, &copied) && copied == len;
}

static void describe_wide(const void *p, char *out, size_t out_size)
{
    WCHAR buf[64];
    size_t i;

    if (!p || !safe_read(p, buf, sizeof(buf) - sizeof(WCHAR)))
    {
        _snprintf_s(out, out_size, _TRUNCATE, "<unreadable %p>", p);
        return;
    }
    buf[63] = 0;
    for (i = 0; i < 63 && buf[i]; i++)
    {
        out[i] = (buf[i] >= 0x20 && buf[i] < 0x7f) ? (char)buf[i] : '?';
    }
    out[i] = 0;
}

static void log_module_bases_once(void)
{
    static LONG done = 0;
    static const wchar_t *mods[] = {
        L"Engine.exe",
        L"GameClient.dll",
        L"GameServer.dll",
        L"ZNetwork.dll",
        L"ZNetwork_orig.dll",
    };
    size_t i;

    if (InterlockedExchange(&done, 1) != 0)
    {
        return;
    }
    for (i = 0; i < sizeof(mods) / sizeof(mods[0]); i++)
    {
        HMODULE h = GetModuleHandleW(mods[i]);
        log_line("module %-18ls base=%p", mods[i], (void *)h);
    }
}

// --------------------------------------------------- original entry points

typedef unsigned char(__cdecl *fn_is_flag)(void);
typedef void(__cdecl *fn_set_cb)(void *);

static fn_is_flag orig_IS_HOST;
static fn_is_flag orig_IS_DEDI;
static fn_set_cb orig_set_game_start;
static fn_set_cb orig_set_host_game_start;
static fn_set_cb orig_set_game_load_complete;
static fn_set_cb orig_set_host_game_load_complete;
static fn_set_cb orig_set_notify_error;
static fn_set_cb orig_set_notify_error_exit;
static fn_set_cb orig_set_login_data;
static fn_set_cb orig_set_my_user_info;
static fn_set_cb orig_set_my_nickname_create;
static fn_set_cb orig_set_nickname_in_use;
static fn_set_cb orig_set_login_fail;
static fn_set_cb orig_set_dedi_login_data;
static void *orig_send_game_leave;
static void *orig_send_game_load_percent;

// Callbacks the client registered; our thunks tail-jump to these.
static void *cb_game_start;
static void *cb_host_game_start;
static void *cb_game_load_complete;
static void *cb_host_game_load_complete;
static void *cb_notify_error;
static void *cb_notify_error_exit;
static void *cb_login_data;
static void *cb_my_user_info;
static void *cb_my_nickname_create;
static void *cb_nickname_in_use;
static void *cb_login_fail;
static void *cb_dedi_login_data;

// ------------------------------------------------------------ log helpers
//
// The thunk passes a pointer to the saved PUSHAD frame, so both register and
// stack arguments are visible (some callbacks are __thiscall/__fastcall and
// pass data in ECX/EDX rather than on the stack). Layout of the frame:
//   [0]=EDI [1]=ESI [2]=EBP [3]=ESP [4]=EBX [5]=EDX [6]=ECX [7]=EAX
//   [8]=return address (caller)   [9+]=stack args
#define R_EDX(f) ((f)[5])
#define R_ECX(f) ((f)[6])
#define R_EAX(f) ((f)[7])
#define R_RET(f) ((f)[8])
#define R_ARG(f, n) ((f)[9 + (n)])

static void __cdecl log_game_start(void **f)
{
    char name[80];
    describe_wide(R_ARG(f, 0), name, sizeof(name));
    log_line("On_GAME_START   map=\"%s\" len=%d bIntrude=%d  (caller=%p)",
             name, (int)(short)(INT_PTR)R_ARG(f, 1), (int)(INT_PTR)R_ARG(f, 2),
             R_RET(f));
}

static void __cdecl log_host_game_start(void **f)
{
    char name[80];
    describe_wide(R_ARG(f, 0), name, sizeof(name));
    log_line("On_HOST_GAME_START map=\"%s\" len=%d bIntrude=%d  (caller=%p)",
             name, (int)(short)(INT_PTR)R_ARG(f, 1), (int)(INT_PTR)R_ARG(f, 2),
             R_RET(f));
}

// Re-assert the ZNetwork "isHost" authority byte (manager ZM at DAT_1025db48,
// +0x350) after the world finishes loading. The world-load session reset clears
// it, and the NotifyRoundBegin stub only fires the host begin-round callback
// (which ends the server-side warmup and spawns the player) when this byte is 1.
#define ZM_ISHOST_RVA (0x25db48 + 0x350)
static int g_force_is_host = 1;

static void force_is_host_authority(void)
{
    if (g_force_is_host && g_orig)
    {
        volatile unsigned char *p =
            (unsigned char *)((UINT_PTR)g_orig + ZM_ISHOST_RVA);
        if (*p != 1)
        {
            *p = 1;
            log_line("proxy: forced isHost (ZM+0x350)=1 @%p", (void *)p);
        }
    }
}

static void __cdecl log_game_load_complete(void **f)
{
    log_line("On_GAME_LOAD_COMPLETE  (caller=%p)", R_RET(f));
    force_is_host_authority();
}

static void __cdecl log_host_game_load_complete(void **f)
{
    log_line("On_HOST_GAME_LOAD_COMPLETE  (caller=%p)", R_RET(f));
    force_is_host_authority();
}

static void __cdecl log_notify_error(void **f)
{
    log_line("On_NOTIFY_ERROR      ecx=%p edx=%p a0=%p a1=%p  (caller=%p)",
             R_ECX(f), R_EDX(f), R_ARG(f, 0), R_ARG(f, 1), R_RET(f));
}

static void __cdecl log_notify_error_exit(void **f)
{
    log_line("On_NOTIFY_ERROR_EXIT ecx=%p edx=%p a0=%p a1=%p  (caller=%p)",
             R_ECX(f), R_EDX(f), R_ARG(f, 0), R_ARG(f, 1), R_RET(f));
}

// Hex-dumps up to `n` bytes at a plausible pointer, and flags any embedded
// UTF-16 run (a nickname would show up as one). Used to inspect the account
// structs the login callbacks receive, whose field layout is unknown.
static void dump_ptr(const char *tag, void *p, SIZE_T n)
{
    unsigned char buf[64];
    char hex[64 * 3 + 1];
    char asc[64 + 1];
    char wide[40];
    SIZE_T i, w = 0;

    if ((ULONG_PTR)p < 0x10000 || n > sizeof(buf))
    {
        return;
    }
    if (!safe_read(p, buf, n))
    {
        return;
    }

    for (i = 0; i < n; i++)
    {
        _snprintf_s(hex + i * 3, 4, 3, "%02x ", buf[i]);
        asc[i] = (buf[i] >= 0x20 && buf[i] < 0x7f) ? (char)buf[i] : '.';
    }
    asc[n] = 0;

    // UTF-16LE run: ascii byte followed by 0x00.
    for (i = 0; i + 1 < n && w + 1 < sizeof(wide); i += 2)
    {
        if (buf[i] >= 0x20 && buf[i] < 0x7f && buf[i + 1] == 0)
        {
            wide[w++] = (char)buf[i];
        }
        else if (buf[i] == 0 && buf[i + 1] == 0 && w > 0)
        {
            break;
        }
    }
    wide[w] = 0;

    log_line("    %s [%p] %s| %s%s%s", tag, p, hex, asc,
             w ? "  utf16=" : "", wide);
}

// Login/identity callbacks: the account data that decides the profile. These
// are __thiscall/__fastcall (the data pointer is in ECX, sometimes EDX), so log
// the registers as well as the stack args and hex-dump every one that looks
// like a pointer. Following one level catches a struct whose first field is a
// name buffer.
static void log_login_cb(const char *label, void **f)
{
    void *ecx = R_ECX(f), *edx = R_EDX(f);
    void *a0 = R_ARG(f, 0), *a1 = R_ARG(f, 1);
    void *deref = NULL;

    log_line("%s  ecx=%p edx=%p a0=%p a1=%p  (caller=%p)",
             label, ecx, edx, a0, a1, R_RET(f));
    dump_ptr("ecx  ", ecx, 48);
    dump_ptr("edx  ", edx, 48);
    dump_ptr("a0   ", a0, 48);
    if ((ULONG_PTR)ecx >= 0x10000 && safe_read(ecx, &deref, sizeof(deref)))
    {
        dump_ptr("*ecx ", deref, 48);
    }
}

static void __cdecl log_login_data(void **f) { log_login_cb("On_LOGIN_DATA       ", f); }
static void __cdecl log_my_user_info(void **f) { log_login_cb("On_MY_USER_INFO     ", f); }
static void __cdecl log_my_nickname_create(void **f) { log_login_cb("On_MY_NICKNAME_CREATE", f); }
static void __cdecl log_nickname_in_use(void **f) { log_login_cb("On_NICKNAME_IN_USE  ", f); }
static void __cdecl log_login_fail(void **f) { log_login_cb("On_LOGIN_FAIL       ", f); }
static void __cdecl log_dedi_login_data(void **f) { log_login_cb("On_DEDI_LOGIN_DATA  ", f); }

static void __cdecl log_send_game_leave(void **f)
{
    log_line("Send_GAME_LEAVE      ecx=%p a0=%p  (caller=%p)",
             R_ECX(f), R_ARG(f, 0), R_RET(f));
}

static void __cdecl log_send_game_load_percent(void **f)
{
    log_line("Send_GAME_LOAD_PERCENT ecx=%p pct=%d  (caller=%p)",
             R_ECX(f), (int)(INT_PTR)R_ARG(f, 0), R_RET(f));
}

// ------------------------------------------------------------------ thunks
//
// After `pushad; pushfd`, [esp] is the saved flags and [esp+4] is the start of
// the PUSHAD register frame (EDI..EAX), followed at [esp+36] by the return
// address and at [esp+40] by the original stack args. The logger receives that
// frame pointer so it can read both registers and stack arguments.

#define NAKED_THUNK(thunk_name, logger, target)    \
    __declspec(naked) static void thunk_name(void) \
    {                                              \
        __asm { pushad }                            \
        __asm { pushfd }                           \
        __asm { lea eax, [esp + 4] }                           \
        __asm { push eax }                         \
        __asm { call logger }                        \
        __asm {add esp, 4} __asm { popfd }         \
        __asm { popad }                             \
        __asm { jmp dword ptr[target] }            \
    }

NAKED_THUNK(thunk_game_start, log_game_start, cb_game_start)
NAKED_THUNK(thunk_host_game_start, log_host_game_start, cb_host_game_start)
NAKED_THUNK(thunk_game_load_complete, log_game_load_complete, cb_game_load_complete)
NAKED_THUNK(thunk_host_game_load_complete, log_host_game_load_complete, cb_host_game_load_complete)
NAKED_THUNK(thunk_notify_error, log_notify_error, cb_notify_error)
NAKED_THUNK(thunk_notify_error_exit, log_notify_error_exit, cb_notify_error_exit)
NAKED_THUNK(thunk_login_data, log_login_data, cb_login_data)
NAKED_THUNK(thunk_my_user_info, log_my_user_info, cb_my_user_info)
NAKED_THUNK(thunk_my_nickname_create, log_my_nickname_create, cb_my_nickname_create)
NAKED_THUNK(thunk_nickname_in_use, log_nickname_in_use, cb_nickname_in_use)
NAKED_THUNK(thunk_login_fail, log_login_fail, cb_login_fail)
NAKED_THUNK(thunk_dedi_login_data, log_dedi_login_data, cb_dedi_login_data)
// These two are exports the client calls, not callbacks it registers, so the
// exported symbol is itself the thunk and tail-jumps into ZNetwork_orig.
#define EXPORTED_NAKED_THUNK(export_name, logger, target)                  \
    __declspec(dllexport) __declspec(naked) void __cdecl export_name(void) \
    {                                                                      \
        __asm { pushad }                                                    \
        __asm { pushfd }                                                   \
        __asm { lea eax, [esp + 4] }                                                   \
        __asm { push eax }                                                 \
        __asm { call logger }                                                \
        __asm {add esp, 4} __asm { popfd }                                 \
        __asm { popad }                                                     \
        __asm { jmp dword ptr[target] }                                    \
    }

EXPORTED_NAKED_THUNK(Send_GAME_LEAVE, log_send_game_leave, orig_send_game_leave)
EXPORTED_NAKED_THUNK(Send_GAME_LOAD_PERCENT, log_send_game_load_percent, orig_send_game_load_percent)

// -------------------------------------------------------- exported wrappers

// IS_HOST and IS_DEDI take no arguments and return a byte, so these are safe to
// wrap in plain C. They are polled, so only transitions are logged.
__declspec(dllexport) unsigned char __cdecl IS_HOST(void)
{
    static int last = -1;
    unsigned char value = orig_IS_HOST ? orig_IS_HOST() : 0;
    if ((int)value != last)
    {
        last = (int)value;
        log_line("IS_HOST -> %u", value);
    }
    return value;
}

__declspec(dllexport) unsigned char __cdecl IS_DEDI(void)
{
    static int last = -1;
    unsigned char value = orig_IS_DEDI ? orig_IS_DEDI() : 0;
    if ((int)value != last)
    {
        last = (int)value;
        log_line("IS_DEDI -> %u", value);
    }
    return value;
}

#define SETTER_WRAPPER(export_name, slot, thunk, orig_setter, label) \
    __declspec(dllexport) void __cdecl export_name(void *cb)         \
    {                                                                \
        log_module_bases_once();                                     \
        log_line("register " label " cb=%p", cb);                    \
        slot = cb;                                                   \
        if (orig_setter)                                             \
        {                                                            \
            orig_setter(cb ? (void *)thunk : NULL);                  \
        }                                                            \
    }

SETTER_WRAPPER(Set_On_GAME_START_Callback, cb_game_start,
               thunk_game_start, orig_set_game_start, "On_GAME_START")
SETTER_WRAPPER(Set_On_HOST_GAME_START_Callback, cb_host_game_start,
               thunk_host_game_start, orig_set_host_game_start, "On_HOST_GAME_START")
SETTER_WRAPPER(Set_On_GAME_LOAD_COMPLETE_Callback, cb_game_load_complete,
               thunk_game_load_complete, orig_set_game_load_complete, "On_GAME_LOAD_COMPLETE")
SETTER_WRAPPER(Set_On_HOST_GAME_LOAD_COMPLETE_Callback, cb_host_game_load_complete,
               thunk_host_game_load_complete, orig_set_host_game_load_complete, "On_HOST_GAME_LOAD_COMPLETE")
SETTER_WRAPPER(Set_On_NOTIFY_ERROR_Callback, cb_notify_error,
               thunk_notify_error, orig_set_notify_error, "On_NOTIFY_ERROR")
SETTER_WRAPPER(Set_On_NOTIFY_ERROR_EXIT_Callback, cb_notify_error_exit,
               thunk_notify_error_exit, orig_set_notify_error_exit, "On_NOTIFY_ERROR_EXIT")
SETTER_WRAPPER(Set_On_LOGIN_DATA_Callback, cb_login_data,
               thunk_login_data, orig_set_login_data, "On_LOGIN_DATA")
SETTER_WRAPPER(Set_On_MY_USER_INFO_Callback, cb_my_user_info,
               thunk_my_user_info, orig_set_my_user_info, "On_MY_USER_INFO")
SETTER_WRAPPER(Set_On_MY_NICKNAME_CREATE_Callback, cb_my_nickname_create,
               thunk_my_nickname_create, orig_set_my_nickname_create, "On_MY_NICKNAME_CREATE")
SETTER_WRAPPER(Set_On_NICKNAME_IN_USE_Callback, cb_nickname_in_use,
               thunk_nickname_in_use, orig_set_nickname_in_use, "On_NICKNAME_IN_USE")
SETTER_WRAPPER(Set_On_LOGIN_FAIL_Callback, cb_login_fail,
               thunk_login_fail, orig_set_login_fail, "On_LOGIN_FAIL")
SETTER_WRAPPER(Set_On_DEDI_LOGIN_DATA_Callback, cb_dedi_login_data,
               thunk_dedi_login_data, orig_set_dedi_login_data, "On_DEDI_LOGIN_DATA")

// ----------------------------------------------------------------- startup

static void resolve_originals(void)
{
    orig_IS_HOST = (fn_is_flag)GetProcAddress(g_orig, "IS_HOST");
    orig_IS_DEDI = (fn_is_flag)GetProcAddress(g_orig, "IS_DEDI");
    orig_set_game_start = (fn_set_cb)GetProcAddress(g_orig, "Set_On_GAME_START_Callback");
    orig_set_host_game_start = (fn_set_cb)GetProcAddress(g_orig, "Set_On_HOST_GAME_START_Callback");
    orig_set_game_load_complete = (fn_set_cb)GetProcAddress(g_orig, "Set_On_GAME_LOAD_COMPLETE_Callback");
    orig_set_host_game_load_complete = (fn_set_cb)GetProcAddress(g_orig, "Set_On_HOST_GAME_LOAD_COMPLETE_Callback");
    orig_set_notify_error = (fn_set_cb)GetProcAddress(g_orig, "Set_On_NOTIFY_ERROR_Callback");
    orig_set_notify_error_exit = (fn_set_cb)GetProcAddress(g_orig, "Set_On_NOTIFY_ERROR_EXIT_Callback");
    orig_set_login_data = (fn_set_cb)GetProcAddress(g_orig, "Set_On_LOGIN_DATA_Callback");
    orig_set_my_user_info = (fn_set_cb)GetProcAddress(g_orig, "Set_On_MY_USER_INFO_Callback");
    orig_set_my_nickname_create = (fn_set_cb)GetProcAddress(g_orig, "Set_On_MY_NICKNAME_CREATE_Callback");
    orig_set_nickname_in_use = (fn_set_cb)GetProcAddress(g_orig, "Set_On_NICKNAME_IN_USE_Callback");
    orig_set_login_fail = (fn_set_cb)GetProcAddress(g_orig, "Set_On_LOGIN_FAIL_Callback");
    orig_set_dedi_login_data = (fn_set_cb)GetProcAddress(g_orig, "Set_On_DEDI_LOGIN_DATA_Callback");
    orig_send_game_leave = (void *)GetProcAddress(g_orig, "Send_GAME_LEAVE");
    orig_send_game_load_percent = (void *)GetProcAddress(g_orig, "Send_GAME_LOAD_PERCENT");
}

static BOOL load_original(HMODULE self)
{
    WCHAR path[MAX_PATH];
    DWORD len = GetModuleFileNameW(self, path, MAX_PATH);
    WCHAR *slash;

    if (len == 0 || len >= MAX_PATH)
    {
        return FALSE;
    }

    slash = wcsrchr(path, L'\\');
    if (!slash)
    {
        return FALSE;
    }
    slash[1] = 0;

    if (wcslen(path) + wcslen(LOG_FILE_NAME) < MAX_PATH)
    {
        wcscpy_s(g_log_path, MAX_PATH, path);
        wcscat_s(g_log_path, MAX_PATH, LOG_FILE_NAME);
    }

    if (wcslen(path) + wcslen(CONSOLE_LOG_NAME) < MAX_PATH)
    {
        wcscpy_s(g_console_log_path, MAX_PATH, path);
        wcscat_s(g_console_log_path, MAX_PATH, CONSOLE_LOG_NAME);
    }

    if (wcslen(path) + wcslen(CAPTURE_SENTINEL_NAME) < MAX_PATH)
    {
        wcscpy_s(g_sentinel_path, MAX_PATH, path);
        wcscat_s(g_sentinel_path, MAX_PATH, CAPTURE_SENTINEL_NAME);
    }

    if (wcslen(path) + wcslen(ORIG_DLL_NAME) >= MAX_PATH)
    {
        return FALSE;
    }
    wcscat_s(path, MAX_PATH, ORIG_DLL_NAME);

    g_orig = LoadLibraryW(path);
    return g_orig != NULL;
}

BOOL APIENTRY DllMain(HMODULE self, DWORD reason, LPVOID reserved)
{
    (void)reserved;

    if (reason == DLL_PROCESS_ATTACH)
    {
        DisableThreadLibraryCalls(self);
        InitializeCriticalSection(&g_log_lock);
        g_log_ready = 1;

        if (!load_original(self))
        {
            g_log_ready = 0;
            DeleteCriticalSection(&g_log_lock);
            return FALSE;
        }

        resolve_originals();
        {
            char buf[8];
            if (GetEnvironmentVariableA("FEAR_FORCE_ISHOST", buf, sizeof(buf)) && buf[0] == '0')
            {
                g_force_is_host = 0;
            }
        }
        if (g_sentinel_path[0] != 0 && GetFileAttributesW(g_sentinel_path) != INVALID_FILE_ATTRIBUTES)
        {
            capture_client_stdout();
            log_line("client stdout capture enabled (sentinel present)");
        }
        log_line("---- proxy attached, ZNetwork_orig loaded at %p (force_is_host=%d) ----", g_orig, g_force_is_host);
    }
    else if (reason == DLL_PROCESS_DETACH)
    {
        if (g_log_ready)
        {
            log_line("---- proxy detached ----");
            g_log_ready = 0;
            DeleteCriticalSection(&g_log_lock);
        }
    }

    return TRUE;
}
