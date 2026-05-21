#include "client_bridge.h"

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static void gofreerdp_set_error(gofreerdp_context* ctx, const char* message) {
	if (!ctx)
		return;
	if (!message)
		message = "unknown error";
	snprintf(ctx->last_error, sizeof(ctx->last_error), "%s", message);
}

static void gofreerdp_format_last_error(freerdp* instance, gofreerdp_context* ctx) {
	if (!ctx)
		return;
	if (!instance || !instance->context) {
		gofreerdp_set_error(ctx, "freerdp instance is not initialized");
		return;
	}

	UINT32 code = freerdp_get_last_error(instance->context);
	const char* name = freerdp_get_last_error_name(code);
	const char* detail = freerdp_get_last_error_string(code);
	if (!name)
		name = "FREERDP_ERROR_UNKNOWN";
	if (!detail)
		detail = "unknown error";
	snprintf(ctx->last_error, sizeof(ctx->last_error), "%s: %s", name, detail);
}

static void gofreerdp_snapshot_reset(gofreerdp_context* ctx) {
	if (!ctx)
		return;
	pthread_mutex_lock(&ctx->snapshot_mutex);
	free(ctx->snapshot);
	ctx->snapshot = NULL;
	ctx->snapshot_size = 0;
	ctx->snapshot_width = 0;
	ctx->snapshot_height = 0;
	ctx->snapshot_stride = 0;
	ctx->snapshot_ready = FALSE;
	pthread_mutex_unlock(&ctx->snapshot_mutex);
}

static BOOL gofreerdp_snapshot_update(gofreerdp_context* ctx) {
	if (!ctx)
		return FALSE;

	rdpGdi* gdi = ctx->common.context.gdi;
	if (!gdi || !gdi->primary_buffer || (gdi->width <= 0) || (gdi->height <= 0) || (gdi->stride == 0))
		return FALSE;

	const size_t size = ((size_t)gdi->stride) * ((size_t)gdi->height);
	pthread_mutex_lock(&ctx->snapshot_mutex);
	if (ctx->snapshot_size != size) {
		BYTE* next = (BYTE*)realloc(ctx->snapshot, size);
		if (!next) {
			pthread_mutex_unlock(&ctx->snapshot_mutex);
			return FALSE;
		}
		ctx->snapshot = next;
		ctx->snapshot_size = size;
	}

	memcpy(ctx->snapshot, gdi->primary_buffer, size);
	ctx->snapshot_width = (UINT32)gdi->width;
	ctx->snapshot_height = (UINT32)gdi->height;
	ctx->snapshot_stride = gdi->stride;
	ctx->snapshot_ready = TRUE;
	pthread_mutex_unlock(&ctx->snapshot_mutex);
	return TRUE;
}

static BOOL gofreerdp_gfx_pipeline_init(rdpContext* context, RdpgfxClientContext* gfx) {
	if (!context || !context->gdi || !gfx)
		return FALSE;

	gofreerdp_context* ctx = (gofreerdp_context*)context;
	if (ctx->gfx_gdi_initialized)
		return TRUE;

	if (!gdi_graphics_pipeline_init(context->gdi, gfx)) {
		gofreerdp_set_error(ctx, "failed to initialize GDI graphics pipeline");
		return FALSE;
	}

	ctx->gfx = gfx;
	ctx->gfx_gdi_initialized = TRUE;
	return TRUE;
}

static void gofreerdp_gfx_pipeline_uninit(rdpContext* context) {
	if (!context)
		return;

	gofreerdp_context* ctx = (gofreerdp_context*)context;
	if (!ctx->gfx_gdi_initialized || !context->gdi || !ctx->gfx)
		return;

	gdi_graphics_pipeline_uninit(context->gdi, ctx->gfx);
	ctx->gfx = NULL;
	ctx->gfx_gdi_initialized = FALSE;
}

static BOOL gofreerdp_begin_paint(rdpContext* context) {
	if (!context || !context->gdi || !context->gdi->primary || !context->gdi->primary->hdc ||
		!context->gdi->primary->hdc->hwnd || !context->gdi->primary->hdc->hwnd->invalid)
		return FALSE;

	context->gdi->primary->hdc->hwnd->invalid->null = TRUE;
	return TRUE;
}

static BOOL gofreerdp_end_paint(rdpContext* context) {
	if (!context || !context->gdi || !context->gdi->primary || !context->gdi->primary->hdc)
		return FALSE;

	HGDI_DC hdc = context->gdi->primary->hdc;
	if (!hdc->hwnd)
		return gofreerdp_snapshot_update((gofreerdp_context*)context);

	HGDI_WND hwnd = hdc->hwnd;
	if (!hwnd->invalid || context->gdi->suppressOutput || hwnd->invalid->null || hwnd->ninvalid < 1)
		return TRUE;

	BOOL updated = gofreerdp_snapshot_update((gofreerdp_context*)context);
	hwnd->invalid->null = TRUE;
	hwnd->ninvalid = 0;
	return updated;
}

static BOOL gofreerdp_desktop_resize(rdpContext* context) {
	if (!context || !context->gdi || !context->settings)
		return FALSE;

	if (!gdi_resize(context->gdi,
			freerdp_settings_get_uint32(context->settings, FreeRDP_DesktopWidth),
			freerdp_settings_get_uint32(context->settings, FreeRDP_DesktopHeight)))
		return FALSE;

	gofreerdp_snapshot_reset((gofreerdp_context*)context);
	return TRUE;
}

static void gofreerdp_on_channel_connected(void* context, const ChannelConnectedEventArgs* e) {
	freerdp_client_OnChannelConnectedEventHandler(context, e);
	if (!context || !e || !e->name || !e->pInterface)
		return;

	if (strcmp(e->name, RDPGFX_DVC_CHANNEL_NAME) == 0)
		(void)gofreerdp_gfx_pipeline_init((rdpContext*)context, (RdpgfxClientContext*)e->pInterface);
}

static void gofreerdp_on_channel_disconnected(void* context,
		const ChannelDisconnectedEventArgs* e) {
	if (context && e && e->name && (strcmp(e->name, RDPGFX_DVC_CHANNEL_NAME) == 0))
		gofreerdp_gfx_pipeline_uninit((rdpContext*)context);
	freerdp_client_OnChannelDisconnectedEventHandler(context, e);
}

static BOOL gofreerdp_pre_connect(freerdp* instance) {
	if (!instance || !instance->context || !instance->context->settings)
		return FALSE;

	rdpContext* context = instance->context;
	rdpSettings* settings = context->settings;
	if (!freerdp_settings_set_bool(settings, FreeRDP_CertificateCallbackPreferPEM, TRUE))
		return FALSE;
#ifdef _WIN32
	if (!freerdp_settings_set_uint32(settings, FreeRDP_OsMajorType, OSMAJORTYPE_WINDOWS))
		return FALSE;
	if (!freerdp_settings_set_uint32(settings, FreeRDP_OsMinorType, OSMINORTYPE_WINDOWS_NT))
		return FALSE;
#elif defined(__APPLE__)
	if (!freerdp_settings_set_uint32(settings, FreeRDP_OsMajorType, OSMAJORTYPE_OSX))
		return FALSE;
	if (!freerdp_settings_set_uint32(settings, FreeRDP_OsMinorType, OSMINORTYPE_MACINTOSH))
		return FALSE;
#else
	if (!freerdp_settings_set_uint32(settings, FreeRDP_OsMajorType, OSMAJORTYPE_UNIX))
		return FALSE;
	if (!freerdp_settings_set_uint32(settings, FreeRDP_OsMinorType, OSMINORTYPE_NATIVE_XSERVER))
		return FALSE;
#endif
	if (PubSub_SubscribeChannelConnected(context->pubSub, gofreerdp_on_channel_connected) < 0)
		return FALSE;
	if (PubSub_SubscribeChannelDisconnected(context->pubSub, gofreerdp_on_channel_disconnected) < 0)
		return FALSE;
	return TRUE;
}

static BOOL gofreerdp_post_connect(freerdp* instance) {
	if (!instance || !instance->context || !instance->context->update)
		return FALSE;

	if (!gdi_init(instance, PIXEL_FORMAT_BGRA32))
		return FALSE;

	rdpContext* context = instance->context;
	gofreerdp_context* ctx = (gofreerdp_context*)context;
	context->update->BeginPaint = gofreerdp_begin_paint;
	context->update->EndPaint = gofreerdp_end_paint;
	context->update->DesktopResize = gofreerdp_desktop_resize;
	if (ctx->gfx)
		(void)gofreerdp_gfx_pipeline_init(context, ctx->gfx);

	ctx->connected = TRUE;
	gofreerdp_snapshot_reset(ctx);
	gofreerdp_set_error(ctx, "");
	return TRUE;
}

static void gofreerdp_post_disconnect(freerdp* instance) {
	if (!instance || !instance->context)
		return;

	rdpContext* context = instance->context;
	gofreerdp_context* ctx = (gofreerdp_context*)context;
	ctx->connected = FALSE;
	gofreerdp_gfx_pipeline_uninit(context);
	PubSub_UnsubscribeChannelConnected(context->pubSub, gofreerdp_on_channel_connected);
	PubSub_UnsubscribeChannelDisconnected(context->pubSub, gofreerdp_on_channel_disconnected);
	if (context->gdi)
		gdi_free(instance);
	gofreerdp_snapshot_reset(ctx);
}

static BOOL gofreerdp_context_new(freerdp* instance, rdpContext* context) {
	(void)instance;
	gofreerdp_context* ctx = (gofreerdp_context*)context;
	ctx->insecure = FALSE;
	ctx->snapshot = NULL;
	ctx->snapshot_size = 0;
	ctx->snapshot_width = 0;
	ctx->snapshot_height = 0;
	ctx->snapshot_stride = 0;
	ctx->snapshot_ready = FALSE;
	ctx->connected = FALSE;
	ctx->gfx = NULL;
	ctx->gfx_gdi_initialized = FALSE;
	ctx->last_error[0] = '\0';
	return pthread_mutex_init(&ctx->snapshot_mutex, NULL) == 0;
}

static void gofreerdp_context_free(freerdp* instance, rdpContext* context) {
	(void)instance;
	gofreerdp_context* ctx = (gofreerdp_context*)context;
	if (!ctx)
		return;
	free(ctx->snapshot);
	ctx->snapshot = NULL;
	pthread_mutex_destroy(&ctx->snapshot_mutex);
}

static BOOL gofreerdp_client_global_init(void) {
	return TRUE;
}

static void gofreerdp_client_global_uninit(void) {
	return;
}

static DWORD gofreerdp_verify_certificate(freerdp* instance, const char* host, UINT16 port,
		const char* common_name, const char* subject, const char* issuer,
		const char* fingerprint, DWORD flags) {
	(void)host;
	(void)port;
	(void)common_name;
	(void)subject;
	(void)issuer;
	(void)fingerprint;
	(void)flags;

	gofreerdp_context* ctx = (gofreerdp_context*)instance->context;
	return (ctx && ctx->insecure) ? 1 : 0;
}

static DWORD gofreerdp_verify_changed_certificate(freerdp* instance, const char* host, UINT16 port,
		const char* common_name, const char* subject, const char* issuer,
		const char* new_fingerprint, const char* old_subject, const char* old_issuer,
		const char* old_fingerprint, DWORD flags) {
	(void)host;
	(void)port;
	(void)common_name;
	(void)subject;
	(void)issuer;
	(void)new_fingerprint;
	(void)old_subject;
	(void)old_issuer;
	(void)old_fingerprint;
	(void)flags;

	gofreerdp_context* ctx = (gofreerdp_context*)instance->context;
	return (ctx && ctx->insecure) ? 1 : 0;
}

static BOOL gofreerdp_client_new(freerdp* instance, rdpContext* context) {
	if (!instance || !context)
		return FALSE;

	instance->PreConnect = gofreerdp_pre_connect;
	instance->PostConnect = gofreerdp_post_connect;
	instance->PostDisconnect = gofreerdp_post_disconnect;
	instance->VerifyCertificateEx = gofreerdp_verify_certificate;
	instance->VerifyChangedCertificateEx = gofreerdp_verify_changed_certificate;
	return TRUE;
}

static void gofreerdp_client_free(freerdp* instance, rdpContext* context) {
	(void)instance;
	(void)context;
}

static int gofreerdp_client_start(rdpContext* context) {
	(void)context;
	return 0;
}

static int gofreerdp_client_entry(RDP_CLIENT_ENTRY_POINTS* pEntryPoints) {
	ZeroMemory(pEntryPoints, sizeof(RDP_CLIENT_ENTRY_POINTS));
	pEntryPoints->Version = RDP_CLIENT_INTERFACE_VERSION;
	pEntryPoints->Size = sizeof(RDP_CLIENT_ENTRY_POINTS_V1);
	pEntryPoints->GlobalInit = gofreerdp_client_global_init;
	pEntryPoints->GlobalUninit = gofreerdp_client_global_uninit;
	pEntryPoints->ContextSize = sizeof(gofreerdp_context);
	pEntryPoints->ClientNew = gofreerdp_client_new;
	pEntryPoints->ClientFree = gofreerdp_client_free;
	pEntryPoints->ClientStart = gofreerdp_client_start;
	pEntryPoints->ClientStop = freerdp_client_common_stop;
	return 0;
}

gofreerdp_client* gofreerdp_client_new_instance(void) {
	RDP_CLIENT_ENTRY_POINTS entry = { 0 };
	gofreerdp_client_entry(&entry);
	rdpContext* context = freerdp_client_context_new(&entry);
	if (!context)
		return NULL;

	gofreerdp_client* client = (gofreerdp_client*)calloc(1, sizeof(gofreerdp_client));
	if (!client) {
		freerdp_client_context_free(context);
		return NULL;
	}
	client->context = context;
	client->started = FALSE;
	client->auth_only = FALSE;
	return client;
}

void gofreerdp_client_free_instance(gofreerdp_client* client) {
	if (!client)
		return;
	if (client->started && client->context)
		freerdp_client_stop(client->context);
	if (client->context)
		freerdp_client_context_free(client->context);
	free(client);
}

static gofreerdp_context* gofreerdp_get_ctx(gofreerdp_client* client) {
	if (!client || !client->context)
		return NULL;
	return (gofreerdp_context*)client->context;
}

static freerdp* gofreerdp_get_instance(gofreerdp_client* client) {
	if (!client || !client->context)
		return NULL;
	return client->context->instance;
}

static int gofreerdp_start_client(gofreerdp_client* client) {
	if (!client || !client->context)
		return 0;
	if (client->started)
		return 1;
	if (freerdp_client_start(client->context) != 0)
		return 0;
	client->started = TRUE;
	return 1;
}

static char* gofreerdp_asprintf(const char* fmt, ...) {
	va_list ap;
	va_start(ap, fmt);
	va_list ap2;
	va_copy(ap2, ap);
	const int len = vsnprintf(NULL, 0, fmt, ap);
	va_end(ap);
	if (len < 0) {
		va_end(ap2);
		return NULL;
	}
	char* out = (char*)calloc((size_t)len + 1, 1);
	if (!out) {
		va_end(ap2);
		return NULL;
	}
	(void)vsnprintf(out, (size_t)len + 1, fmt, ap2);
	va_end(ap2);
	return out;
}

int gofreerdp_configure(gofreerdp_client* client, const char* host, UINT16 port,
		const char* username, const char* password, const char* domain, UINT32 width, UINT32 height,
		UINT32 keyboard_layout, BOOL insecure, BOOL auth_only, BOOL graphics_pipeline, BOOL gfx_h264,
		BOOL gfx_avc444) {
	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	freerdp* instance = gofreerdp_get_instance(client);
	if (!ctx || !instance || !instance->context || !instance->context->settings) {
		gofreerdp_set_error(ctx, "freerdp context is unavailable");
		return 0;
	}

	ctx->insecure = insecure;
	ctx->connected = FALSE;
	ctx->last_error[0] = '\0';
	client->auth_only = auth_only;

	rdpSettings* settings = instance->context->settings;
	char* argv[16] = { 0 };
	int argc = 0;
	argv[argc++] = gofreerdp_asprintf("%s", "gofreerdp");
	argv[argc++] = gofreerdp_asprintf("/v:%s:%u", host, (unsigned)port);
	argv[argc++] = gofreerdp_asprintf("/u:%s", username);
	argv[argc++] = gofreerdp_asprintf("/p:%s", password);
	argv[argc++] = gofreerdp_asprintf("/w:%u", (unsigned)width);
	argv[argc++] = gofreerdp_asprintf("/h:%u", (unsigned)height);
	argv[argc++] = gofreerdp_asprintf("/bpp:%u", 32U);
	argv[argc++] = gofreerdp_asprintf("%s", "/auth-pkg-list:!kerberos,!u2u");
	if (graphics_pipeline && !gfx_h264)
		argv[argc++] = gofreerdp_asprintf("%s", "/gfx:progressive");
	if (domain && domain[0] != '\0')
		argv[argc++] = gofreerdp_asprintf("/d:%s", domain);
	if (insecure)
		argv[argc++] = gofreerdp_asprintf("/cert:ignore");

	BOOL ok = TRUE;
	for (int x = 0; x < argc; x++) {
		if (!argv[x]) {
			ok = FALSE;
			break;
		}
	}
	if (!ok) {
		for (int x = 0; x < argc; x++)
			free(argv[x]);
		gofreerdp_set_error(ctx, "failed to allocate freerdp command line arguments");
		return 0;
	}

	const int parseStatus = freerdp_client_settings_parse_command_line(settings, argc, argv, FALSE);
	for (int x = 0; x < argc; x++)
		free(argv[x]);

	const BOOL progressive = graphics_pipeline && !gfx_h264;
	if (parseStatus < 0 ||
		!freerdp_settings_set_bool(settings, FreeRDP_AutoReconnectionEnabled, FALSE) ||
		!freerdp_settings_set_bool(settings, FreeRDP_GfxH264, gfx_h264) ||
		!freerdp_settings_set_bool(settings, FreeRDP_GfxAVC444, gfx_avc444) ||
		!freerdp_settings_set_bool(settings, FreeRDP_SupportGraphicsPipeline, graphics_pipeline) ||
		!freerdp_settings_set_bool(settings, FreeRDP_GfxProgressive, progressive) ||
		!freerdp_settings_set_bool(settings, FreeRDP_GfxProgressiveV2, progressive) ||
		!freerdp_settings_set_bool(settings, FreeRDP_GfxThinClient, FALSE) ||
		!freerdp_settings_set_bool(settings, FreeRDP_GfxSmallCache, FALSE) ||
		!freerdp_settings_set_uint32(settings, FreeRDP_KeyboardLayout, keyboard_layout) ||
		!freerdp_settings_set_bool(settings, FreeRDP_AuthenticationOnly, auth_only)) {
		gofreerdp_set_error(ctx, "failed to populate freerdp settings");
		return 0;
	}

	if (!freerdp_settings_are_valid(settings)) {
		gofreerdp_set_error(ctx, "freerdp rejected the generated settings");
		return 0;
	}
	return 1;
}

int gofreerdp_probe_auth_only(gofreerdp_client* client) {
	if (!client || !client->context)
		return GOFREERDP_CONNECT_FAILED;
	if (!gofreerdp_start_client(client)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "freerdp_client_start failed");
		return GOFREERDP_CONNECT_FAILED;
	}

	freerdp* instance = gofreerdp_get_instance(client);
	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	if (freerdp_connect(instance)) {
		freerdp_disconnect(instance);
		return GOFREERDP_CONNECT_OK;
	}

	UINT32 lastError = freerdp_get_last_error(instance->context);
	UINT32 errorInfo = freerdp_error_info(instance);
	if ((lastError == FREERDP_ERROR_CONNECT_TRANSPORT_FAILED) && (errorInfo == ERRINFO_SUCCESS)) {
		freerdp_disconnect(instance);
		return GOFREERDP_CONNECT_AUTH_ONLY_OK;
	}

	gofreerdp_format_last_error(instance, ctx);
	freerdp_disconnect(instance);
	return GOFREERDP_CONNECT_FAILED;
}

int gofreerdp_run_persistent(gofreerdp_client* client) {
	if (!client || !client->context)
		return 0;
	if (!gofreerdp_start_client(client)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "freerdp_client_start failed");
		return 0;
	}

	freerdp* instance = gofreerdp_get_instance(client);
	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	if (!freerdp_connect(instance)) {
		gofreerdp_format_last_error(instance, ctx);
		freerdp_disconnect(instance);
		return 0;
	}

	HANDLE handles[MAXIMUM_WAIT_OBJECTS] = { 0 };
	while (!freerdp_shall_disconnect_context(client->context)) {
		DWORD count = freerdp_get_event_handles(client->context, handles, MAXIMUM_WAIT_OBJECTS);
		if (count == 0) {
			gofreerdp_set_error(ctx, "freerdp_get_event_handles failed");
			freerdp_disconnect(instance);
			return 0;
		}

		DWORD status = WaitForMultipleObjects(count, handles, FALSE, INFINITE);
		if (status == WAIT_FAILED) {
			gofreerdp_set_error(ctx, "WaitForMultipleObjects failed");
			freerdp_disconnect(instance);
			return 0;
		}

		if (!freerdp_check_event_handles(client->context)) {
			gofreerdp_format_last_error(instance, ctx);
			freerdp_disconnect(instance);
			return 0;
		}
	}

	freerdp_disconnect(instance);
	return 1;
}

int gofreerdp_abort(gofreerdp_client* client) {
	if (!client || !client->context)
		return 0;
	return freerdp_abort_connect_context(client->context) ? 1 : 0;
}

const char* gofreerdp_state(gofreerdp_client* client) {
	if (!client || !client->context)
		return "UNKNOWN";
	return freerdp_state_string(freerdp_get_state(client->context));
}

int gofreerdp_is_active(gofreerdp_client* client) {
	if (!client || !client->context)
		return 0;
	return freerdp_is_active_state(client->context) ? 1 : 0;
}

int gofreerdp_is_connected(gofreerdp_client* client) {
	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	return (ctx && ctx->connected) ? 1 : 0;
}

const char* gofreerdp_version(void) {
	return freerdp_get_version_string();
}

const char* gofreerdp_error(gofreerdp_client* client) {
	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	if (!ctx || (ctx->last_error[0] == '\0'))
		return NULL;
	return ctx->last_error;
}

size_t gofreerdp_snapshot_size(gofreerdp_client* client, UINT32* width, UINT32* height,
		UINT32* stride) {
	if (width)
		*width = 0;
	if (height)
		*height = 0;
	if (stride)
		*stride = 0;

	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	if (!ctx)
		return 0;

	(void)gofreerdp_snapshot_update(ctx);

	pthread_mutex_lock(&ctx->snapshot_mutex);
	const size_t size = ctx->snapshot_ready ? ctx->snapshot_size : 0;
	if (size > 0) {
		if (width)
			*width = ctx->snapshot_width;
		if (height)
			*height = ctx->snapshot_height;
		if (stride)
			*stride = ctx->snapshot_stride;
	}
	pthread_mutex_unlock(&ctx->snapshot_mutex);
	return size;
}

int gofreerdp_copy_snapshot(gofreerdp_client* client, BYTE* dst, size_t dst_len,
		UINT32* width, UINT32* height, UINT32* stride) {
	if (width)
		*width = 0;
	if (height)
		*height = 0;
	if (stride)
		*stride = 0;

	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	if (!ctx || !dst)
		return 0;

	(void)gofreerdp_snapshot_update(ctx);

	pthread_mutex_lock(&ctx->snapshot_mutex);
	if (!ctx->snapshot_ready || (ctx->snapshot_size == 0) || (dst_len < ctx->snapshot_size)) {
		pthread_mutex_unlock(&ctx->snapshot_mutex);
		return 0;
	}
	memcpy(dst, ctx->snapshot, ctx->snapshot_size);
	if (width)
		*width = ctx->snapshot_width;
	if (height)
		*height = ctx->snapshot_height;
	if (stride)
		*stride = ctx->snapshot_stride;
	pthread_mutex_unlock(&ctx->snapshot_mutex);
	return 1;
}

static int gofreerdp_get_input(gofreerdp_client* client, rdpInput** input) {
	if (input)
		*input = NULL;

	gofreerdp_context* ctx = gofreerdp_get_ctx(client);
	if (!ctx || !client || !client->context || !client->context->input) {
		gofreerdp_set_error(ctx, "freerdp input interface is unavailable");
		return 0;
	}
	if (!ctx->connected || !freerdp_is_active_state(client->context)) {
		gofreerdp_set_error(ctx, "freerdp session is not active");
		return 0;
	}

	if (input)
		*input = client->context->input;
	return 1;
}

UINT32 gofreerdp_rdp_scancode_from_name(const char* name) {
	if (!name || name[0] == '\0')
		return 0;

	const DWORD vk = GetVirtualKeyCodeFromName(name);
	if (vk == VK_NONE)
		return 0;

	return GetVirtualScanCodeFromVirtualKeyCode(vk, WINPR_KBD_TYPE_IBM_ENHANCED);
}

int gofreerdp_send_input_synchronize(gofreerdp_client* client, UINT32 flags) {
	rdpInput* input = NULL;
	if (!gofreerdp_get_input(client, &input))
		return 0;
	if (!freerdp_input_send_synchronize_event(input, flags)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "failed to send synchronize event");
		return 0;
	}
	return 1;
}

int gofreerdp_send_focus_in(gofreerdp_client* client, UINT16 toggle_states) {
	rdpInput* input = NULL;
	if (!gofreerdp_get_input(client, &input))
		return 0;
	if (!freerdp_input_send_focus_in_event(input, toggle_states)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "failed to send focus-in event");
		return 0;
	}
	return 1;
}

int gofreerdp_send_keyboard_scancode(gofreerdp_client* client, UINT32 rdp_scancode, BOOL down,
		BOOL repeat) {
	rdpInput* input = NULL;
	if (!gofreerdp_get_input(client, &input))
		return 0;
	if (!freerdp_input_send_keyboard_event_ex(input, down, repeat, rdp_scancode)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "failed to send keyboard scancode event");
		return 0;
	}
	return 1;
}

int gofreerdp_unicode_input_available(gofreerdp_client* client) {
	if (!client || !client->context || !client->context->settings)
		return 0;

	return freerdp_settings_get_bool(client->context->settings, FreeRDP_UnicodeInput) ? 1 : 0;
}

int gofreerdp_send_unicode_keyboard(gofreerdp_client* client, UINT16 code, BOOL release) {
	rdpInput* input = NULL;
	if (!gofreerdp_get_input(client, &input))
		return 0;

	const UINT16 flags = release ? KBD_FLAGS_RELEASE : 0;
	if (!freerdp_input_send_unicode_keyboard_event(input, flags, code)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "failed to send unicode keyboard event");
		return 0;
	}
	return 1;
}

int gofreerdp_send_mouse(gofreerdp_client* client, UINT16 flags, UINT16 x, UINT16 y) {
	rdpInput* input = NULL;
	if (!gofreerdp_get_input(client, &input))
		return 0;
	if (!freerdp_input_send_mouse_event(input, flags, x, y)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "failed to send mouse event");
		return 0;
	}
	return 1;
}

int gofreerdp_send_extended_mouse(gofreerdp_client* client, UINT16 flags, UINT16 x, UINT16 y) {
	rdpInput* input = NULL;
	if (!gofreerdp_get_input(client, &input))
		return 0;
	if (!freerdp_input_send_extended_mouse_event(input, flags, x, y)) {
		gofreerdp_set_error(gofreerdp_get_ctx(client), "failed to send extended mouse event");
		return 0;
	}
	return 1;
}
