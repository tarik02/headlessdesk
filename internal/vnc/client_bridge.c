#include "client_bridge.h"

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <rfb/rfbproto.h>

extern void goVNCFramebuffer(uintptr_t handle, const uint8_t* data, int width, int height, int bytesPerPixel, int bigEndian, int redShift, int greenShift, int blueShift, int redMax, int greenMax, int blueMax);

static int govnc_handle_tag;

static uintptr_t govnc_get_handle(rfbClient* client) {
	return (uintptr_t)rfbClientGetClientData(client, &govnc_handle_tag);
}

static char* govnc_get_password(rfbClient* client) {
	const char* password = (const char*)rfbClientGetClientData(client, client);
	if (password == NULL) {
		return strdup("");
	}
	return strdup(password);
}

static rfbBool govnc_malloc_framebuffer(rfbClient* client) {
	const size_t bytes_per_pixel = client->format.bitsPerPixel / 8;
	const size_t size = (size_t)client->width * (size_t)client->height * bytes_per_pixel;
	uint8_t* buffer = calloc(size, 1);
	if (buffer == NULL) {
		return FALSE;
	}
	free(client->frameBuffer);
	client->frameBuffer = buffer;
	return TRUE;
}

static void govnc_got_framebuffer_update(rfbClient* client, int x, int y, int w, int h) {
	(void)x;
	(void)y;
	(void)w;
	(void)h;

	if (client->frameBuffer == NULL) {
		return;
	}

	goVNCFramebuffer(
		govnc_get_handle(client),
		client->frameBuffer,
		client->width,
		client->height,
		client->format.bitsPerPixel / 8,
		client->format.bigEndian,
		client->format.redShift,
		client->format.greenShift,
		client->format.blueShift,
		client->format.redMax,
		client->format.greenMax,
		client->format.blueMax
	);
}

rfbClient* govnc_new_client(const char* host, int port, const char* password, int shared, uintptr_t handle) {
	rfbClient* client = rfbGetClient(8, 3, 4);
	if (client == NULL) {
		return NULL;
	}

	client->appData.shareDesktop = shared ? TRUE : FALSE;
	client->appData.viewOnly = FALSE;
	client->appData.forceTrueColour = TRUE;
	client->appData.requestedDepth = 24;
	client->appData.encodingsString = "tight zrle hextile raw";
	client->serverHost = strdup(host);
	client->serverPort = port;
	client->GetPassword = govnc_get_password;
	client->MallocFrameBuffer = govnc_malloc_framebuffer;
	client->GotFrameBufferUpdate = govnc_got_framebuffer_update;
	client->canHandleNewFBSize = TRUE;

	rfbClientSetClientData(client, &govnc_handle_tag, (void*)handle);
	rfbClientSetClientData(client, client, strdup(password == NULL ? "" : password));

	int argc = 1;
	char* argv[] = { "headlessdesk", NULL };
	if (!rfbInitClient(client, &argc, argv)) {
		govnc_cleanup_client(client);
		return NULL;
	}

	SendFramebufferUpdateRequest(client, 0, 0, client->width, client->height, FALSE);
	return client;
}

void govnc_close_client(rfbClient* client) {
	if (client != NULL) {
		rfbCloseSocket(client->sock);
	}
}

void govnc_cleanup_client(rfbClient* client) {
	if (client == NULL) {
		return;
	}

	void* password = rfbClientGetClientData(client, client);
	if (password != NULL) {
		free(password);
	}
	free(client->frameBuffer);
	client->frameBuffer = NULL;
	rfbClientCleanup(client);
}

rfbBool govnc_wait_for_message(rfbClient* client, unsigned int usecs) {
	return WaitForMessage(client, usecs) >= 0;
}

rfbBool govnc_handle_server_message(rfbClient* client) {
	return HandleRFBServerMessage(client);
}

rfbBool govnc_send_frame_request(rfbClient* client, int incremental) {
	return SendFramebufferUpdateRequest(client, 0, 0, client->width, client->height, incremental ? TRUE : FALSE);
}

rfbBool govnc_send_key(rfbClient* client, uint32_t key, int down) {
	return SendKeyEvent(client, key, down ? TRUE : FALSE);
}

rfbBool govnc_send_pointer(rfbClient* client, int x, int y, int mask) {
	return SendPointerEvent(client, x, y, mask);
}
