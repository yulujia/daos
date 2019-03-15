
/**
 * (C) Copyright 2015-2018 Intel Corporation.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * GOVERNMENT LICENSE RIGHTS-OPEN SOURCE SOFTWARE
 * The Government's rights to use, modify, reproduce, release, perform, display,
 * or disclose this software are subject to the terms of the Apache License
 * provided in Contract No. B609815.
 * Any reproduction of computer software, computer software documentation, or
 * portions thereof marked with this legend must also reproduce the markings.
 */

#ifndef __DAOS_ERASURE_CODE_H
#define __DAOS_ERASURE_CODE_H

#include <daos/common.h>

#include <isa-l.h>

#define HIGH_BIT (unsigned long)1 << 63

struct dc_parity {
       int             nr;
       unsigned char   **p_bufs;
};

/**
 * Encode (using ISA-L) a full stripe from the submitted scatter-gather list.
 *
 * \param sgl		[IN]		The SGL containing the user data.
 * \param j		[IN|OUT]	Index for start of SGL sg_iovs.
 * \param k		[IN|OUT]	Index into sg_iovs' io_buf.
 * \param parity	[IN|OUT]	Struct containing parity buffers.
 * \param p_idx		[IN]		Index into parity p_bufs array.
 * \param cs		[IN]		Cell size in bytes.
 * \param dc		[IN]		Number of data cells in stripe.
 * \param pc		[IN]		Number of parity cells in stripe.
 * \param g_tbls	[IN]		Galois field encoding table (stored
 * 					in object class structure).
 */
int
daos_encode_full_stripe(daos_sg_list_t *sgl, unsigned int *j, size_t *k,
                        struct dc_parity *parity, int p_idx, int cs, int dc,
			int pc, unsigned char **g_tbls);


#endif
