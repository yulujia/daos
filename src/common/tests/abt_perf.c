/**
 * (C) Copyright 2017 Intel Corporation.
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
#define D_LOGFAC	DD_FAC(tests)

#include <abt.h>
#include <daos/common.h>
#include <getopt.h>

static unsigned long	abt_cntr;
static unsigned long	abt_created;
static bool		abt_waiting;
static bool		abt_exiting;
static bool		abt_in_ult;

static ABT_pool		abt_pool;
static ABT_cond		abt_cond;
static ABT_mutex	abt_lock;
static ABT_xstream	abt_xstream;
static char		*abt_name;

static int		opt_secs;
static int		opt_threads = 1;
static int		cr_opc;

static void
abt_thread_1(void *arg)
{
	ABT_mutex_lock(abt_lock);
	abt_created--;
	if (abt_waiting)
		ABT_cond_broadcast(abt_cond);
	ABT_mutex_unlock(abt_lock);
}

/**
 * Create UTLs for a few seconds, concurrent creation is always lower
 * than @opt_threads.
 */
static void
abt_ult_create_rate(void)
{
	double		then;
	double		now;
	double		prt;
	int		rc;

	prt = then = ABT_get_wtime();
	while (1) {
		if (!abt_exiting) {
			now = ABT_get_wtime();
			if (now - then >= opt_secs)
				abt_exiting = true;
		}

		if (abt_exiting) {
			if (abt_created == 0)
				break;

			ABT_thread_yield();
			continue;
		}

		ABT_mutex_lock(abt_lock);
		if (abt_created >= opt_threads) {
			abt_waiting = true;
			ABT_cond_wait(abt_cond, abt_lock);
		}
		abt_waiting = false;
		abt_created++;
		abt_cntr++;

		ABT_mutex_unlock(abt_lock);

		rc = ABT_thread_create(abt_pool, abt_thread_1, NULL,
				       ABT_THREAD_ATTR_NULL, NULL);
		if (rc != ABT_SUCCESS) {
			printf("ABT thread create failed: %d\n", rc);
			return;
		}

		if (now - prt >= 1) {
			printf("Created %lu threads in %d seconds\n",
			       abt_cntr, (int)(now - then));
			prt = now;
		}
	}
	printf("ABT creation rate = %lu/sec.\n", abt_cntr / opt_secs);
}

static void
abt_thread_2(void *arg)
{
	ABT_mutex_lock(abt_lock);
	while (!abt_exiting) {
		abt_cntr++;
		ABT_mutex_unlock(abt_lock);

		ABT_thread_yield();

		ABT_mutex_lock(abt_lock);
	}
	abt_created--;
	ABT_mutex_unlock(abt_lock);
}

/**
 * Create @opt_threads ULTs, and scheduling all of them for @opt_secs seconds.
 */
static void
abt_sched_rate(void)
{
	double		then;
	double		now;
	int		rc;

	then = ABT_get_wtime();
	while (1) {
		if (!abt_exiting) {
			now = ABT_get_wtime();
			if (now - then >= opt_secs)
				abt_exiting = true;
		}

		if (abt_exiting && abt_created == 0)
			break;

		if (abt_exiting || (abt_created >= opt_threads)) {
			ABT_thread_yield();
			continue;
		}

		ABT_mutex_lock(abt_lock);
		abt_created++;
		ABT_mutex_unlock(abt_lock);

		rc = ABT_thread_create(abt_pool, abt_thread_2, NULL,
				       ABT_THREAD_ATTR_NULL, NULL);
		if (rc != ABT_SUCCESS) {
			printf("ABT thread create failed: %d\n", rc);
			ABT_mutex_lock(abt_lock);
			abt_created--;
			abt_exiting = true;
			ABT_mutex_unlock(abt_lock);
		}
	}
	printf("ABT scheduling rate = %lu/sec.\n", abt_cntr / opt_secs);
}

enum {
	CR_MUTEX,
	CR_COND,
	CR_EVENTUAL,
};

static void
abt_lock_create_rate(void *arg)
{
	ABT_mutex	mutex;
	ABT_cond	cond;
	ABT_eventual	eventual;
	double		then;
	double		now;
	int		rc;

	then = ABT_get_wtime();
	while (1) {
		if (!abt_exiting) {
			now = ABT_get_wtime();
			if (now - then >= opt_secs)
				abt_exiting = true;
		}

		if (abt_exiting)
			break;

		switch (cr_opc) {
		case CR_MUTEX:
			rc = ABT_mutex_create(&mutex);
			assert(rc == ABT_SUCCESS);
			ABT_mutex_free(&mutex);
			break;

		case CR_COND:
			rc = ABT_cond_create(&cond);
			assert(rc == ABT_SUCCESS);
			ABT_cond_free(&cond);
			break;

		case CR_EVENTUAL:
			rc = ABT_eventual_create(sizeof(int), &eventual);
			assert(rc == ABT_SUCCESS);
			ABT_eventual_free(&eventual);
			break;
		}
		abt_cntr++;
	}
	printf("ABT %s creation rate = %lu/sec.\n",
		abt_name, abt_cntr / opt_secs);

	ABT_mutex_lock(abt_lock);
	if (abt_waiting) {
		ABT_cond_broadcast(abt_cond);
		abt_waiting = false;
	}
	ABT_mutex_unlock(abt_lock);
}

static void
abt_reset(void)
{
	abt_cntr	= 0;
	abt_created	= 0;
	abt_exiting	= false;
	abt_waiting	= false;
}

static struct option abt_ops[] = {
	/**
	 * test-id:
	 * c = create test
	 * s = schedule test
	 */
	{ "test",	required_argument,	NULL,	't'	},
	/**
	 * if test-id is 'c', it is the number of concurrent creation
	 * if test-id is 's', it is the total number of running ULTs
	 */
	{ "num",	required_argument,	NULL,	'n'	},
	/**
	 * test duration in seconds.
	 */
	{ "sec",	required_argument,	NULL,	's'	},
	{ "ult",	no_argument,		NULL,	'u'	},
};

int
main(int argc, char **argv)
{
	char	test_id = 0;
	int	rc;

	while ((rc = getopt_long(argc, argv, "t:n:s:u",
				 abt_ops, NULL)) != -1) {
		switch (rc) {
		default:
			fprintf(stderr, "unknown opc=%c\n", rc);
			exit(-1);
		case 't':
			test_id = *optarg;
			break;
		case 'n':
			opt_threads = atoi(optarg);
			break;
		case 's':
			opt_secs = atoi(optarg);
			break;
		case 'u':
			abt_in_ult = true;
			break;
		}
	}

	if (opt_secs == 0) {
		printf("invalid sec=%s\n", argv[1]);
		return -1;
	}

	if (opt_threads == 0) {
		printf("invalid ABT threads=%s\n", argv[2]);
		return -1;
	}

	printf("Create ABT threads for %d seconds, concur=%d\n",
	       opt_secs, opt_threads);

	rc = ABT_init(0, NULL);
	if (rc != ABT_SUCCESS) {
		printf("ABT init failed: %d\n", rc);
		return -1;
	}

	rc = ABT_xstream_self(&abt_xstream);
	if (rc != ABT_SUCCESS) {
		printf("ABT get self xstream failed: %d\n", rc);
		return -1;
	}

	rc = ABT_xstream_get_main_pools(abt_xstream, 1, &abt_pool);
	if (rc != ABT_SUCCESS) {
		printf("ABT pool get failed: %d\n", rc);
		return -1;
	}

	rc = ABT_cond_create(&abt_cond);
	if (rc != ABT_SUCCESS) {
		printf("ABT cond create failed: %d\n", rc);
		return -1;
	}

	rc = ABT_mutex_create(&abt_lock);
	if (rc != ABT_SUCCESS) {
		printf("ABT mutex create failed: %d\n", rc);
		return -1;
	}

	switch (test_id) {
	default:
		break;
	case 'c':
		printf("ULT create rate test (concur=%d, secs=%d)\n",
		       opt_threads, opt_secs);
		abt_ult_create_rate();
		goto out;
	case 's':
		printf("ULT scheduling rate test (ULTs=%d, secs=%d)\n",
		       opt_threads, opt_secs);
		abt_sched_rate();
		goto out;
	case 'm':
		printf("mutex creation rate test (secs=%d)\n",
		       opt_secs);
		cr_opc = CR_MUTEX;
		abt_name = "mutex";
		break;

	case 'e':
		printf("eventual creation rate test within ULT (secs=%d)\n",
		       opt_secs);
		cr_opc = CR_EVENTUAL;
		abt_name = "eventual";
		break;
	case 'd':
		printf("condition creation rate test within ULT (secs=%d)\n",
		       opt_secs);
		cr_opc = CR_COND;
		abt_name = "cond";
		break;
	}

	if (abt_in_ult) {
		printf("run test in ULT\n");
		abt_waiting = true;
		rc = ABT_thread_create(abt_pool, abt_lock_create_rate, NULL,
				       ABT_THREAD_ATTR_NULL, NULL);

		ABT_mutex_lock(abt_lock);
		if (abt_waiting)
			ABT_cond_wait(abt_cond, abt_lock);
		ABT_mutex_unlock(abt_lock);

	} else {
		abt_lock_create_rate(NULL);
	}
out:
	abt_reset();

	ABT_mutex_free(&abt_lock);
	ABT_cond_free(&abt_cond);
	ABT_finalize();
	return 0;
}
