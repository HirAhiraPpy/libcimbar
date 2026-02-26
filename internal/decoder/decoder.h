/* This code is subject to the terms of the Mozilla Public License, v.2.0. http://mozilla.org/MPL/2.0/. */
#ifndef CIMBAR_RECV_CGO_H
#define CIMBAR_RECV_CGO_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

// Import functions from cimbar_recv_js
unsigned cimbard_get_report(unsigned char* buff, unsigned maxlen);
unsigned cimbard_get_debug(unsigned char* buff, unsigned maxlen);
int cimbard_get_bufsize();
int cimbard_scan_extract_decode(const unsigned char* imgdata, unsigned imgw, unsigned imgh, int format, unsigned char* bufspace, unsigned bufsize);
int64_t cimbard_fountain_decode(const unsigned char* buffer, unsigned size);
unsigned cimbard_get_filesize(uint32_t id);
int cimbard_get_filename(uint32_t id, char* filename, unsigned fnsize);
int cimbard_get_decompress_bufsize();
int cimbard_decompress_read(uint32_t id, unsigned char* buffer, unsigned size);
int cimbard_configure_decode(int mode_val);

#ifdef __cplusplus
}
#endif

#endif // CIMBAR_RECV_CGO_H
