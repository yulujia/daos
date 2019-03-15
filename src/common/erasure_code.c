
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
 * or disclose this software are subject to the terms of the Apache License as
 * provided in Contract No. B609815.
 * Any reproduction of computer software, computer software documentation, or
 * portions thereof marked with this legend must also reproduce the markings.
 */

#define DDSUBSYS	DDFAC(common)

#include "daos/erasure_code.h"

/* Parameters converted to ISA-L terms:
 * k - data cells
 * m - parity cells
 */
static int
daos_ec_encode_data(int k, int m, int len, unsigned char **g_tbls,
		    unsigned char **data, unsigned char **coding)
{
	unsigned char	*encode_matrix;
	int 		rc = 0; 

	if ( *g_tbls == NULL) {
		D_ALLOC(*g_tbls, 32 * k * m);
		if ( !(*g_tbls)) {
			D_GOTO(failed, rc = -DER_NOMEM);
		}
		D_ALLOC(encode_matrix, (k+m) * k);
		if (!encode_matrix) {
			D_FREE(*g_tbls);
			*g_tbls = NULL;
			D_GOTO(failed, rc = -DER_NOMEM);
		}
		gf_gen_cauchy1_matrix(encode_matrix, k+m, k);
		ec_init_tables(k, m, &encode_matrix[k * k], *g_tbls);
		D_FREE(encode_matrix);

	}
	ec_encode_data(len, k, m, *g_tbls, data, coding);
failed:
	return rc;
}

/* Encode full stripe from SGL */
int
daos_encode_full_stripe(daos_sg_list_t *sgl, unsigned int *j, size_t *k, 
			struct dc_parity *parity, int p_idx, int cs, int sw,
			int pc, unsigned char **g_tbls)
{
	unsigned char *data[sw];
	unsigned char *ldata[sw];
	int i, lcnt = 0;
	int rc = 0;
	
	for (i = 0; i < sw; i++) 
		if (sgl->sg_iovs[*j].iov_len - *k >= cs) {
			unsigned char* from =
				(unsigned char*)sgl->sg_iovs[*j].iov_buf;
			data[i] = &from[*k];
			*k += cs;
			if (*k == sgl->sg_iovs[*j].iov_len) {
				*k = 0; (*j)++;
			}
		} else {
			int cp_cnt = 0;
			ldata[lcnt] = (unsigned char*)malloc(cs);
			if (ldata[lcnt] == NULL)
				D_GOTO(out, rc = -DER_NOMEM);
			while (cp_cnt < cs) {
				int cp_amt = sgl->sg_iovs[*j].iov_len-*k <
					cs - cp_cnt ?
					sgl->sg_iovs[*j].iov_len-*k :
					cs - cp_cnt;
				unsigned char* from = sgl->sg_iovs[*j].iov_buf;
				memcpy(&ldata[lcnt][cp_cnt], &from[*k], cp_amt);
				if (sgl->sg_iovs[*j].iov_len-*k < cs - cp_cnt) {
					 *k = 0;
					(*j)++;
				} else
					*k += cp_amt;
				cp_cnt += cp_amt;
			}
			data[i] = ldata[lcnt++];
		}
				       
	rc = daos_ec_encode_data(sw, pc, cs, g_tbls, data,
				 &parity->p_bufs[p_idx]);
out:
	for (i = 0; i < lcnt; i++)
		free(ldata[i]);
	return rc;
}

