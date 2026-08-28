/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

#ifndef CGO_HELPERS_H
#define CGO_HELPERS_H

#include <osdp.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// Forward declarations for callback wrappers
extern int eventCallbackWrapper(void *data, int pd, struct osdp_event *event);
extern int commandCallbackWrapper(void *data, struct osdp_cmd *cmd);
extern void logCallbackWrapper(int pd, int level, char *msg, char *file, unsigned long line);
extern int channelSendWrapper(void *data, uint8_t *buf, int len);
extern int channelRecvWrapper(void *data, uint8_t *buf, int len);
extern void channelFlushWrapper(void *data);

// Forward declarations for completion callback wrappers
extern void cpCommandCompletionWrapper(void *arg, int pd, struct osdp_cmd *cmd, int status);
extern void pdEventCompletionWrapper(void *arg, struct osdp_event *ev, int status);

// Forward declarations for file transfer callback wrappers
extern int fileOpenWrapper(void *arg, int file_id, uint32_t *size);
extern int fileReadWrapper(void *arg, void *buf, uint32_t size, uint32_t offset);
extern int fileWriteWrapper(void *arg, void *buf, uint32_t size, uint32_t offset);
extern int fileCloseWrapper(void *arg);

// uintptr_to_ptr fabricates an opaque void* from an integer registry handle.
// Handles are passed through libosdp's void* callback args and never
// dereferenced as pointers. Doing the integer->pointer cast in C avoids Go's
// checkptr, which (correctly, in general) forbids uintptr->unsafe.Pointer in
// Go code; here the "pointer" is only ever converted back to uintptr for a map
// lookup, so the cast is safe.
static inline void *uintptr_to_ptr(uintptr_t v) {
    return (void *)v;
}

// Channel management functions
static inline struct osdp_channel* create_channel(void *data) {
    struct osdp_channel *ch = malloc(sizeof(struct osdp_channel));
    memset(ch, 0, sizeof(*ch));
    ch->data = data;
    ch->send = channelSendWrapper;
    ch->recv = channelRecvWrapper;
    ch->flush = channelFlushWrapper;
    return ch;
}

static inline void free_channel(struct osdp_channel *ch) {
    if (ch) {
        free(ch);
    }
}

// Command helper functions
static inline void set_cmd_output(struct osdp_cmd *cmd, uint8_t output_no, uint8_t control_code, uint16_t timer_count) {
    cmd->output.output_no = output_no;
    cmd->output.control_code = control_code;
    cmd->output.timer_count = timer_count;
}

static inline void set_cmd_led(struct osdp_cmd *cmd, uint8_t reader, uint8_t led_number,
                        uint8_t temp_control, uint8_t temp_on_count, uint8_t temp_off_count,
                        uint8_t temp_on_color, uint8_t temp_off_color, uint16_t temp_timer,
                        uint8_t perm_control, uint8_t perm_on_count, uint8_t perm_off_count,
                        uint8_t perm_on_color, uint8_t perm_off_color, uint16_t perm_timer) {
    cmd->led.reader = reader;
    cmd->led.led_number = led_number;
    cmd->led.temporary.control_code = temp_control;
    cmd->led.temporary.on_count = temp_on_count;
    cmd->led.temporary.off_count = temp_off_count;
    cmd->led.temporary.on_color = temp_on_color;
    cmd->led.temporary.off_color = temp_off_color;
    cmd->led.temporary.timer_count = temp_timer;
    cmd->led.permanent.control_code = perm_control;
    cmd->led.permanent.on_count = perm_on_count;
    cmd->led.permanent.off_count = perm_off_count;
    cmd->led.permanent.on_color = perm_on_color;
    cmd->led.permanent.off_color = perm_off_color;
    cmd->led.permanent.timer_count = perm_timer;
}

static inline void set_cmd_buzzer(struct osdp_cmd *cmd, uint8_t reader, uint8_t control_code,
                           uint8_t on_count, uint8_t off_count, uint8_t count) {
    cmd->buzzer.reader = reader;
    cmd->buzzer.control_code = control_code;
    cmd->buzzer.on_count = on_count;
    cmd->buzzer.off_count = off_count;
    cmd->buzzer.rep_count = count;
}

static inline void set_cmd_text(struct osdp_cmd *cmd, uint8_t reader, uint8_t control_code,
                         uint8_t temp_time, uint8_t offset_row, uint8_t offset_col,
                         char *data, uint8_t length) {
    cmd->text.reader = reader;
    cmd->text.control_code = control_code;
    cmd->text.temp_time = temp_time;
    cmd->text.offset_row = offset_row;
    cmd->text.offset_col = offset_col;
    memcpy(cmd->text.data, data, length);
    cmd->text.length = length;
}

// A CP->PD status command is a request: only the report type matters.
static inline void set_cmd_status(struct osdp_cmd *cmd, uint8_t type) {
    cmd->status.type = type;
    cmd->status.nr_entries = 0;
}

static inline void get_cmd_status(struct osdp_cmd *cmd, uint8_t *type) {
    *type = cmd->status.type;
}

static inline void set_cmd_status_response(struct osdp_cmd *cmd, uint8_t type, int nr_entries, uint8_t *report) {
    cmd->status.type = type;
    cmd->status.nr_entries = nr_entries;
    memcpy(cmd->status.report, report, nr_entries);
}

static inline void set_cmd_mfg(struct osdp_cmd *cmd, uint32_t vendor_code, uint8_t *data, uint8_t length) {
    cmd->mfg.vendor_code = vendor_code;
    cmd->mfg.length = length;
    if (length) memcpy(cmd->mfg.data, data, length);
}

// Event helper functions
// cardread.length is the card data length in *bits*; the data buffer holds the
// ceil(length/8) bytes. Keep the two counts distinct to avoid over-reads.
static inline void get_event_cardread(struct osdp_event *ev, int *reader_no, uint8_t *format, int *direction, uint16_t *length_bits, uint8_t *data) {
    *reader_no = ev->cardread.reader_no;
    *format = ev->cardread.format;
    *direction = ev->cardread.direction;
    *length_bits = ev->cardread.length;
    memcpy(data, ev->cardread.data, (ev->cardread.length + 7) / 8);
}

static inline void get_event_keypress(struct osdp_event *ev, int *reader_no, int *length, uint8_t *data) {
    *reader_no = ev->keypress.reader_no;
    *length = ev->keypress.length;
    memcpy(data, ev->keypress.data, ev->keypress.length);
}

static inline void get_event_mfgrep(struct osdp_event *ev, uint32_t *vendor_code, uint8_t *length, uint8_t *data) {
    *vendor_code = ev->mfgrep.vendor_code;
    *length = ev->mfgrep.length;
    memcpy(data, ev->mfgrep.data, ev->mfgrep.length);
}

static inline void get_event_status(struct osdp_event *ev, uint8_t *type, uint8_t *nr_entries, uint8_t *report) {
    *type = ev->status.type;
    *nr_entries = ev->status.nr_entries;
    memcpy(report, ev->status.report, ev->status.nr_entries);
}

// Notifications reach a CP as an osdp_event and a PD as an osdp_cmd; both carry
// the same struct osdp_notification, so the getters take it directly.
static inline struct osdp_notification *event_notif(struct osdp_event *ev) { return &ev->notif; }
static inline struct osdp_notification *cmd_notif(struct osdp_cmd *cmd) { return &cmd->notif; }

static inline int get_notification_type(struct osdp_notification *n) { return n->type; }

static inline void get_notification_command(struct osdp_notification *n, int *command, int *success) {
    *command = n->command.command;
    *success = n->command.success ? 1 : 0;
}
static inline void get_notification_sc_status(struct osdp_notification *n, int *active, int *scbk_d) {
    *active = n->sc_status.active ? 1 : 0;
    *scbk_d = n->sc_status.scbk_d ? 1 : 0;
}
static inline void get_notification_pd_status(struct osdp_notification *n, int *online) {
    *online = n->pd_status.online ? 1 : 0;
}
static inline void get_notification_mp(struct osdp_notification *n, int *mp_type, int *object_id, uint32_t *total, uint32_t *offset, int *outcome) {
    *mp_type = n->mp.mp_type;
    *object_id = n->mp.object_id;
    *total = n->mp.total;
    *offset = n->mp.offset;
    *outcome = n->mp.outcome;
}
static inline void get_notification_pd_id(struct osdp_notification *n, int *version, int *model, unsigned int *vendor_code, unsigned int *serial_number, unsigned int *firmware_version) {
    *version = n->pd_id.version;
    *model = n->pd_id.model;
    *vendor_code = n->pd_id.vendor_code;
    *serial_number = n->pd_id.serial_number;
    *firmware_version = n->pd_id.firmware_version;
}

// Notification setters: notifications are decode-only in production (libosdp
// synthesizes them), so these exist to construct osdp_event fixtures in tests.
static inline void set_notification_command(struct osdp_event *ev, int command, int success) {
    ev->notif.type = OSDP_NOTIFICATION_COMMAND;
    ev->notif.command.command = command;
    ev->notif.command.success = success;
}
static inline void set_notification_sc_status(struct osdp_event *ev, int active, int scbk_d) {
    ev->notif.type = OSDP_NOTIFICATION_SC_STATUS;
    ev->notif.sc_status.active = active;
    ev->notif.sc_status.scbk_d = scbk_d;
}
static inline void set_notification_pd_status(struct osdp_event *ev, int online) {
    ev->notif.type = OSDP_NOTIFICATION_PD_STATUS;
    ev->notif.pd_status.online = online;
}
static inline void set_notification_mp(struct osdp_event *ev, int type, enum osdp_mp_msg_type mp_type, int object_id, uint32_t total, uint32_t offset, int outcome) {
    ev->notif.type = type;
    ev->notif.mp.mp_type = mp_type;
    ev->notif.mp.object_id = object_id;
    ev->notif.mp.total = total;
    ev->notif.mp.offset = offset;
    ev->notif.mp.outcome = outcome;
}
static inline void set_notification_pd_id(struct osdp_event *ev, int version, int model, unsigned int vendor_code, unsigned int serial_number, unsigned int firmware_version) {
    ev->notif.type = OSDP_NOTIFICATION_PD_ID;
    ev->notif.pd_id.version = version;
    ev->notif.pd_id.model = model;
    ev->notif.pd_id.vendor_code = vendor_code;
    ev->notif.pd_id.serial_number = serial_number;
    ev->notif.pd_id.firmware_version = firmware_version;
}

static inline void set_event_cardread(struct osdp_event *ev, uint8_t reader_no, uint8_t format, uint8_t direction, int length_bits, uint8_t *data, int data_len) {
    ev->cardread.reader_no = reader_no;
    ev->cardread.format = format;
    ev->cardread.direction = direction;
    ev->cardread.length = length_bits;
    memcpy(ev->cardread.data, data, data_len);
}

static inline void set_event_keypress(struct osdp_event *ev, uint8_t reader_no, uint8_t length, uint8_t *data) {
    ev->keypress.reader_no = reader_no;
    ev->keypress.length = length;
    if (length) memcpy(ev->keypress.data, data, length);
}

static inline void set_event_status_report(struct osdp_event *ev, uint8_t type, uint8_t nr_entries, uint8_t *report) {
    ev->status.type = type;
    ev->status.nr_entries = nr_entries;
    if (nr_entries) memcpy(ev->status.report, report, nr_entries);
}

static inline void set_event_mfgrep(struct osdp_event *ev, uint32_t vendor_code, uint8_t length, uint8_t *data) {
    ev->mfgrep.vendor_code = vendor_code;
    ev->mfgrep.length = length;
    if (length) memcpy(ev->mfgrep.data, data, length);
}

// MFGSTATR and MFGERRR share struct osdp_event_mfgstat on distinct union arms.
static inline void set_event_mfgstat_arm(struct osdp_event_mfgstat *m, uint8_t length, uint8_t *data) {
    m->length = length;
    if (length) memcpy(m->data, data, length);
}
static inline void get_event_mfgstat_arm(struct osdp_event_mfgstat *m, uint8_t *length, uint8_t *data) {
    *length = m->length;
    if (m->length) memcpy(data, m->data, m->length);
}
static inline void set_event_mfgstatr(struct osdp_event *ev, uint8_t length, uint8_t *data) { set_event_mfgstat_arm(&ev->mfgstatr, length, data); }
static inline void get_event_mfgstatr(struct osdp_event *ev, uint8_t *length, uint8_t *data) { get_event_mfgstat_arm(&ev->mfgstatr, length, data); }
static inline void set_event_mfgerrr(struct osdp_event *ev, uint8_t length, uint8_t *data) { set_event_mfgstat_arm(&ev->mfgerrr, length, data); }
static inline void get_event_mfgerrr(struct osdp_event *ev, uint8_t *length, uint8_t *data) { get_event_mfgstat_arm(&ev->mfgerrr, length, data); }

static inline void set_cmd_biomatch(struct osdp_cmd *cmd, uint8_t reader, int type, int format, uint8_t quality, uint16_t length, uint8_t *data) {
    cmd->biomatch.reader = reader; cmd->biomatch.type = type; cmd->biomatch.format = format;
    cmd->biomatch.quality = quality; cmd->biomatch.length = length;
    if (length) memcpy(cmd->biomatch.data, data, length);
}
static inline void get_cmd_biomatch(struct osdp_cmd *cmd, uint8_t *reader, int *type, int *format, uint8_t *quality, uint16_t *length, uint8_t *data) {
    *reader = cmd->biomatch.reader; *type = cmd->biomatch.type; *format = cmd->biomatch.format;
    *quality = cmd->biomatch.quality; *length = cmd->biomatch.length;
    if (cmd->biomatch.length) memcpy(data, cmd->biomatch.data, cmd->biomatch.length);
}
static inline void set_event_biomatchr(struct osdp_event *ev, uint8_t reader, int status, uint8_t score) {
    ev->biomatchr.reader = reader; ev->biomatchr.status = status; ev->biomatchr.score = score;
}
static inline void get_event_biomatchr(struct osdp_event *ev, uint8_t *reader, int *status, uint8_t *score) {
    *reader = ev->biomatchr.reader; *status = ev->biomatchr.status; *score = ev->biomatchr.score;
}

static inline void set_cmd_bioread(struct osdp_cmd *cmd, uint8_t reader, int type, int format, uint8_t quality) {
    cmd->bioread.reader = reader; cmd->bioread.type = type; cmd->bioread.format = format; cmd->bioread.quality = quality;
}
static inline void get_cmd_bioread(struct osdp_cmd *cmd, uint8_t *reader, int *type, int *format, uint8_t *quality) {
    *reader = cmd->bioread.reader; *type = cmd->bioread.type; *format = cmd->bioread.format; *quality = cmd->bioread.quality;
}
static inline void set_event_bioreadr(struct osdp_event *ev, uint8_t reader, int status, int type, uint8_t quality, uint16_t length, uint8_t *data) {
    ev->bioreadr.reader = reader; ev->bioreadr.status = status; ev->bioreadr.type = type;
    ev->bioreadr.quality = quality; ev->bioreadr.length = length;
    if (length) memcpy(ev->bioreadr.data, data, length);
}
static inline void get_event_bioreadr(struct osdp_event *ev, uint8_t *reader, int *status, int *type, uint8_t *quality, uint16_t *length, uint8_t *data) {
    *reader = ev->bioreadr.reader; *status = ev->bioreadr.status; *type = ev->bioreadr.type;
    *quality = ev->bioreadr.quality; *length = ev->bioreadr.length;
    if (ev->bioreadr.length) memcpy(data, ev->bioreadr.data, ev->bioreadr.length);
}

static inline void set_cmd_pivdata(struct osdp_cmd *cmd, uint8_t *oid, uint8_t element, uint8_t offset) {
    cmd->pivdata.oid[0] = oid[0]; cmd->pivdata.oid[1] = oid[1]; cmd->pivdata.oid[2] = oid[2];
    cmd->pivdata.element = element; cmd->pivdata.offset = offset;
}
static inline void get_cmd_pivdata(struct osdp_cmd *cmd, uint8_t *oid, uint8_t *element, uint8_t *offset) {
    oid[0] = cmd->pivdata.oid[0]; oid[1] = cmd->pivdata.oid[1]; oid[2] = cmd->pivdata.oid[2];
    *element = cmd->pivdata.element; *offset = cmd->pivdata.offset;
}

// piv_reply backs PIVDATAR, GENAUTHR, and CRAUTHR; the event id distinguishes.
static inline void set_event_piv_reply(struct osdp_event *ev, uint16_t length, uint8_t *data) {
    ev->piv_reply.length = length;
    if (length) memcpy(ev->piv_reply.data, data, length);
}
static inline void get_event_piv_reply(struct osdp_event *ev, uint16_t *length, uint8_t *data) {
    *length = ev->piv_reply.length;
    if (ev->piv_reply.length) memcpy(data, ev->piv_reply.data, ev->piv_reply.length);
}

// GENAUTH and CRAUTH share struct osdp_cmd_auth; the command id distinguishes.
static inline void set_cmd_auth(struct osdp_cmd *cmd, uint8_t algorithm, uint8_t key, uint16_t length, uint8_t *data) {
    cmd->auth.algorithm = algorithm; cmd->auth.key = key; cmd->auth.length = length;
    if (length) memcpy(cmd->auth.data, data, length);
}
static inline void get_cmd_auth(struct osdp_cmd *cmd, uint8_t *algorithm, uint8_t *key, uint16_t *length, uint8_t *data) {
    *algorithm = cmd->auth.algorithm; *key = cmd->auth.key; *length = cmd->auth.length;
    if (cmd->auth.length) memcpy(data, cmd->auth.data, cmd->auth.length);
}

// Command parsing helper functions
static inline void get_cmd_output(struct osdp_cmd *cmd, uint8_t *output_no, uint8_t *control_code, uint16_t *timer_count) {
    *output_no = cmd->output.output_no;
    *control_code = cmd->output.control_code;
    *timer_count = cmd->output.timer_count;
}

static inline void set_cmd_tdset(struct osdp_cmd *cmd, uint16_t year, uint8_t month, uint8_t day, uint8_t hour, uint8_t minute, uint8_t second) {
    cmd->tdset.year = year; cmd->tdset.month = month; cmd->tdset.day = day;
    cmd->tdset.hour = hour; cmd->tdset.minute = minute; cmd->tdset.second = second;
}
static inline void get_cmd_tdset(struct osdp_cmd *cmd, uint16_t *year, uint8_t *month, uint8_t *day, uint8_t *hour, uint8_t *minute, uint8_t *second) {
    *year = cmd->tdset.year; *month = cmd->tdset.month; *day = cmd->tdset.day;
    *hour = cmd->tdset.hour; *minute = cmd->tdset.minute; *second = cmd->tdset.second;
}

static inline void get_cmd_led(struct osdp_cmd *cmd, uint8_t *reader, uint8_t *led_number,
                        uint8_t *temp_control, uint8_t *temp_on_count, uint8_t *temp_off_count,
                        uint8_t *temp_on_color, uint8_t *temp_off_color, uint16_t *temp_timer,
                        uint8_t *perm_control, uint8_t *perm_on_count, uint8_t *perm_off_count,
                        uint8_t *perm_on_color, uint8_t *perm_off_color, uint16_t *perm_timer) {
    *reader = cmd->led.reader;
    *led_number = cmd->led.led_number;
    *temp_control = cmd->led.temporary.control_code;
    *temp_on_count = cmd->led.temporary.on_count;
    *temp_off_count = cmd->led.temporary.off_count;
    *temp_on_color = cmd->led.temporary.on_color;
    *temp_off_color = cmd->led.temporary.off_color;
    *temp_timer = cmd->led.temporary.timer_count;
    *perm_control = cmd->led.permanent.control_code;
    *perm_on_count = cmd->led.permanent.on_count;
    *perm_off_count = cmd->led.permanent.off_count;
    *perm_on_color = cmd->led.permanent.on_color;
    *perm_off_color = cmd->led.permanent.off_color;
    *perm_timer = cmd->led.permanent.timer_count;
}

static inline void get_cmd_buzzer(struct osdp_cmd *cmd, uint8_t *reader, uint8_t *control_code,
                           uint8_t *on_count, uint8_t *off_count, uint8_t *rep_count) {
    *reader = cmd->buzzer.reader;
    *control_code = cmd->buzzer.control_code;
    *on_count = cmd->buzzer.on_count;
    *off_count = cmd->buzzer.off_count;
    *rep_count = cmd->buzzer.rep_count;
}

static inline void get_cmd_text(struct osdp_cmd *cmd, uint8_t *reader, uint8_t *control_code,
                         uint8_t *temp_time, uint8_t *offset_row, uint8_t *offset_col,
                         uint8_t *length, char *data) {
    *reader = cmd->text.reader;
    *control_code = cmd->text.control_code;
    *temp_time = cmd->text.temp_time;
    *offset_row = cmd->text.offset_row;
    *offset_col = cmd->text.offset_col;
    *length = cmd->text.length;
    memcpy(data, cmd->text.data, cmd->text.length);
}

static inline void get_cmd_mfg(struct osdp_cmd *cmd, uint32_t *vendor_code, uint8_t *length, uint8_t *data) {
    *vendor_code = cmd->mfg.vendor_code;
    *length = cmd->mfg.length;
    memcpy(data, cmd->mfg.data, cmd->mfg.length);
}

static inline void set_cmd_keyset(struct osdp_cmd *cmd, uint8_t type, uint8_t length, uint8_t *data) {
    cmd->keyset.type = type;
    cmd->keyset.length = length;
    if (length) memcpy(cmd->keyset.data, data, length);
}

static inline void set_cmd_comset(struct osdp_cmd *cmd, uint8_t address, uint32_t baud_rate) {
    cmd->comset.address = address;
    cmd->comset.baud_rate = baud_rate;
}

static inline void set_cmd_file_tx(struct osdp_cmd *cmd, int id, uint32_t flags) {
    cmd->file_tx.id = id;
    cmd->file_tx.flags = flags;
}

static inline void get_cmd_keyset(struct osdp_cmd *cmd, uint8_t *type, uint8_t *length, uint8_t *data) {
    *type = cmd->keyset.type;
    *length = cmd->keyset.length;
    memcpy(data, cmd->keyset.data, cmd->keyset.length);
}

static inline void get_cmd_comset(struct osdp_cmd *cmd, uint8_t *address, uint32_t *baud_rate) {
    *address = cmd->comset.address;
    *baud_rate = cmd->comset.baud_rate;
}

static inline void get_cmd_file_tx(struct osdp_cmd *cmd, int *id, uint32_t *flags) {
    *id = cmd->file_tx.id;
    *flags = cmd->file_tx.flags;
}


// Callback setter functions
static inline void set_event_callback_wrapper(void *ctx, void *data) {
    osdp_cp_set_event_callback((osdp_t *)ctx, eventCallbackWrapper, data);
}

static inline void set_command_callback_wrapper(void *ctx, void *data) {
    osdp_pd_set_command_callback((osdp_t *)ctx, commandCallbackWrapper, data);
}

static inline void set_command_completion_wrapper(void *ctx, void *data) {
    osdp_cp_set_command_completion_callback((osdp_t *)ctx,
        (cp_command_completion_callback_t)cpCommandCompletionWrapper, data);
}

static inline void set_event_completion_wrapper(void *ctx, void *data) {
    osdp_pd_set_event_completion_callback((osdp_t *)ctx,
        (pd_event_completion_callback_t)pdEventCompletionWrapper, data);
}

static inline void set_log_callback_wrapper(void) {
    osdp_set_log_callback((osdp_log_callback_fn_t)logCallbackWrapper);
}

#endif // CGO_HELPERS_H