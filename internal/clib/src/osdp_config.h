/*
 * Copyright (c) 2020-2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

#ifndef _OSDP_CONFIG_H_
#define _OSDP_CONFIG_H_

/**
 * @brief The following macros are defined defined from the variable in cmake
 * files. All @XXX@ are replaced by the value of XXX as resolved by cmake.
 */
#define PROJECT_VERSION                "4.0.0"
/* Human-visible version. Equals PROJECT_VERSION when released; carries the
 * pre-release marker and git position (e.g. "4.0.0-dev.37+ga1b2c3d") while a
 * release cycle is prepared but unpublished. This is what osdp_get_version()
 * returns. */
#define LIBOSDP_VERSION_STR            "4.0.0-dev.337+g50c6824"
#define PROJECT_NAME                   "libosdp"
#define GIT_BRANCH                     "detached"
#define GIT_REV                        "v3.2.0-337-g50c6824"
#define GIT_TAG                        ""
#define GIT_DIFF                       ""
#define REPO_ROOT                      "/home/sidcha/work/osdp/libosdp-go/libosdp"

#endif /* _OSDP_CONFIG_H_ */
