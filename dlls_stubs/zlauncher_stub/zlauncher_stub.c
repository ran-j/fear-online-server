#include <windows.h>
#include <stdbool.h>
#include <stdint.h>

typedef void (__cdecl *VoidCallback)(void);
typedef void (__cdecl *FailCallback)(int);

typedef struct LauncherData {
    uint8_t connected;
    uint8_t login_ok;
    uint8_t reserved[62];
} LauncherData;

static LauncherData g_launcher_data = {0};
static VoidCallback g_connected_callback = NULL;
static VoidCallback g_disconnected_callback = NULL;
static VoidCallback g_login_data_callback = NULL;
static FailCallback g_login_fail_callback = NULL;
static VoidCallback g_block_ip_callback = NULL;

BOOL WINAPI DllMain(HINSTANCE instance, DWORD reason, LPVOID reserved) {
    (void)instance;
    (void)reason;
    (void)reserved;
    return TRUE;
}

__declspec(dllexport) void __cdecl init(void) {
    g_launcher_data.connected = 0;
    g_launcher_data.login_ok = 0;
}

__declspec(dllexport) void __cdecl destoryed(void) {
    g_launcher_data.connected = 0;
    g_launcher_data.login_ok = 0;
}

__declspec(dllexport) void __cdecl Tick(void) {
}

__declspec(dllexport) LauncherData *__cdecl GetLauncherData(void) {
    return &g_launcher_data;
}

__declspec(dllexport) char __cdecl LoginLauncher(
    const wchar_t *login_server_ip,
    short login_server_port,
    const wchar_t *login_type,
    const wchar_t *account_id,
    const wchar_t *password_or_token
) {
    (void)login_server_ip;
    (void)login_server_port;
    (void)login_type;
    (void)account_id;
    (void)password_or_token;

    g_launcher_data.connected = 1;
    g_launcher_data.login_ok = 1;

    if (g_connected_callback != NULL) {
        g_connected_callback();
    }

    if (g_login_data_callback != NULL) {
        g_login_data_callback();
    }

    return 1;
}

__declspec(dllexport) void __cdecl DisConnect(void) {
    g_launcher_data.connected = 0;
    if (g_disconnected_callback != NULL) {
        g_disconnected_callback();
    }
}

__declspec(dllexport) void __cdecl Set_OnIoConnectedCallback(VoidCallback callback) {
    g_connected_callback = callback;
}

__declspec(dllexport) void __cdecl Set_OnIoDisonnectedCallback(VoidCallback callback) {
    g_disconnected_callback = callback;
}

__declspec(dllexport) void __cdecl Set_On_LOGIN_DATA_Callback(VoidCallback callback) {
    g_login_data_callback = callback;
}

__declspec(dllexport) void __cdecl Set_On_LOGIN_FAIL_Callback(FailCallback callback) {
    g_login_fail_callback = callback;
}

__declspec(dllexport) void __cdecl Set_On_BLOCK_IP_Callback(VoidCallback callback) {
    g_block_ip_callback = callback;
}
