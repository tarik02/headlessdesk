#ifndef GOFREERDP_CLIENT_BRIDGE_H
#define GOFREERDP_CLIENT_BRIDGE_H

#include <pthread.h>
#include <stddef.h>
#include <stdint.h>

#include <freerdp/client.h>
#include <freerdp/constants.h>
#include <freerdp/freerdp.h>
#include <freerdp/client/rdpgfx.h>
#include <freerdp/gdi/gdi.h>
#include <freerdp/gdi/gfx.h>
#include <freerdp/input.h>
#include <freerdp/settings.h>
#include <winpr/input.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
	rdpClientContext common;
	BOOL insecure;
	pthread_mutex_t snapshot_mutex;
	BYTE* snapshot;
	size_t snapshot_size;
	UINT32 snapshot_width;
	UINT32 snapshot_height;
	UINT32 snapshot_stride;
	BOOL snapshot_ready;
	BOOL connected;
	RdpgfxClientContext* gfx;
	BOOL gfx_gdi_initialized;
	char last_error[512];
} gofreerdp_context;

typedef struct {
	rdpContext* context;
	BOOL started;
	BOOL auth_only;
} gofreerdp_client;

enum {
	GOFREERDP_CONNECT_FAILED = 0,
	GOFREERDP_CONNECT_OK = 1,
	GOFREERDP_CONNECT_AUTH_ONLY_OK = 2
};

gofreerdp_client* gofreerdp_client_new_instance(void);
void gofreerdp_client_free_instance(gofreerdp_client* client);
int gofreerdp_configure(gofreerdp_client* client, const char* host, UINT16 port,
		const char* username, const char* password, const char* domain, UINT32 width, UINT32 height,
		UINT32 keyboard_layout, BOOL insecure, BOOL auth_only, BOOL graphics_pipeline,
		BOOL gfx_h264, BOOL gfx_avc444);
int gofreerdp_probe_auth_only(gofreerdp_client* client);
int gofreerdp_run_persistent(gofreerdp_client* client);
int gofreerdp_abort(gofreerdp_client* client);
const char* gofreerdp_state(gofreerdp_client* client);
int gofreerdp_is_active(gofreerdp_client* client);
int gofreerdp_is_connected(gofreerdp_client* client);
const char* gofreerdp_version(void);
const char* gofreerdp_error(gofreerdp_client* client);
size_t gofreerdp_snapshot_size(gofreerdp_client* client, UINT32* width, UINT32* height,
		UINT32* stride);
int gofreerdp_copy_snapshot(gofreerdp_client* client, BYTE* dst, size_t dst_len, UINT32* width,
		UINT32* height, UINT32* stride);
UINT32 gofreerdp_rdp_scancode_from_name(const char* name);
int gofreerdp_send_input_synchronize(gofreerdp_client* client, UINT32 flags);
int gofreerdp_send_focus_in(gofreerdp_client* client, UINT16 toggle_states);
int gofreerdp_send_keyboard_scancode(gofreerdp_client* client, UINT32 rdp_scancode, BOOL down,
		BOOL repeat);
int gofreerdp_unicode_input_available(gofreerdp_client* client);
int gofreerdp_send_unicode_keyboard(gofreerdp_client* client, UINT16 code, BOOL release);
int gofreerdp_send_mouse(gofreerdp_client* client, UINT16 flags, UINT16 x, UINT16 y);
int gofreerdp_send_extended_mouse(gofreerdp_client* client, UINT16 flags, UINT16 x, UINT16 y);

#ifdef __cplusplus
}
#endif

#endif
