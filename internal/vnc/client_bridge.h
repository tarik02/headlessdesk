#pragma once

#include <stdint.h>
#include <rfb/rfbclient.h>

rfbClient* govnc_new_client(const char* host, int port, const char* password, int shared, uintptr_t handle);
void govnc_close_client(rfbClient* client);
void govnc_cleanup_client(rfbClient* client);
rfbBool govnc_wait_for_message(rfbClient* client, unsigned int usecs);
rfbBool govnc_handle_server_message(rfbClient* client);
rfbBool govnc_send_frame_request(rfbClient* client, int incremental);
rfbBool govnc_send_key(rfbClient* client, uint32_t key, int down);
rfbBool govnc_send_pointer(rfbClient* client, int x, int y, int mask);
