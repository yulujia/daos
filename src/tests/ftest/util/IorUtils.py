#!/usr/bin/python
'''
    (C) Copyright 2018-2019 Intel Corporation.

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.

    GOVERNMENT LICENSE RIGHTS-OPEN SOURCE SOFTWARE
    The Government's rights to use, modify, reproduce, release, perform, display,
    or disclose this software are subject to the terms of the Apache License as
    provided in Contract No. B609815.
    Any reproduction of computer software, computer software documentation, or
    portions thereof marked with this legend must also reproduce the markings.
    '''
import os, shutil
import subprocess
import json
import itertools

class IorFailed(Exception):
    """Raise if Ior failed"""

def build_ior(basepath):
    from  git import Repo
    """ Pulls the DAOS branch of IOR and builds it """

    HOME = os.path.expanduser("~")
    repo = os.path.abspath(HOME + "/ior-hpc")

    # check if there is pre-existing ior repo.
    if os.path.isdir(repo):
        shutil.rmtree(repo)

    with open(os.path.join(basepath, ".build_vars.json")) as f:
        build_paths = json.load(f)
    daos_dir = build_paths['PREFIX']

    try:
        # pulling daos branch of IOR
        Repo.clone_from("https://github.com/daos-stack/ior-hpc.git", repo, branch='daos')

        cd_cmd = 'cd ' + repo
        bootstrap_cmd = cd_cmd + ' && ./bootstrap '
        configure_cmd = cd_cmd + ' && ./configure --prefix={0} --with-daos={0}'.format(daos_dir)
        make_cmd = cd_cmd + ' &&  make install'

        # building ior
        subprocess.check_call(bootstrap_cmd, shell=True)
        subprocess.check_call(configure_cmd, shell=True)
        subprocess.check_call(make_cmd, shell=True)

    except subprocess.CalledProcessError as e:
        print "<IorBuildFailed> Exception occurred: {0}".format(str(e))
        raise IorFailed("IOR Build process Failed")

def run_ior(client_file, ior_flags, iteration, block_size, transfer_size, pool_uuid, svc_list,
            record_size, stripe_size, stripe_count, async_io, object_class, basepath, slots=1,
            seg_count=1, filename="`uuidgen`", display_output=True):
    """ Running Ior tests
        Function Arguments
        client_file    --client file holding client hostname and slots
        ior_flags      --all ior specific flags
        iteration      --number of iterations for ior run
        block_size     --contiguous bytes to write per task
        transfer_size  --size of transfer in bytes
        pool_uuid      --Daos Pool UUID
        svc_list       --Daos Pool SVCL
        record_size    --Daos Record Size
        stripe_size    --Daos Stripe Size
        stripe_count   --Daos Stripe Count
        async_io       --Concurrent Async IOs
        object_class   --object class
        basepath       --Daos basepath
        slots          --slots on each node
        seg_count      --segment count
        filename       --Container file name
        display_output --print IOR output on console.
    """
    with open(os.path.join(basepath, ".build_vars.json")) as f:
        build_paths = json.load(f)
    orterun_bin = os.path.join(build_paths["OMPI_PREFIX"], "bin/orterun")
    attach_info_path = basepath + "/install/tmp"
    try:

        ior_cmd = orterun_bin + " -N {} --hostfile {} -x DAOS_SINGLETON_CLI=1 " \
                  " -x CRT_ATTACH_INFO_PATH={} ior {} -s {} -i {} -a DAOS -o {} " \
                  " -b {} -t {} --daos.pool={} --daos.svcl={} --daos.recordSize={} --daos.stripeSize={} --daos.stripeCount={} --daos.aios={} --daos.objectClass={} "\
                  .format(slots, client_file, attach_info_path, ior_flags, seg_count, iteration,
                          filename, block_size, transfer_size, pool_uuid,
                          svc_list, record_size, stripe_size,
                          stripe_count, async_io, object_class)
        if display_output:
            print ("ior_cmd: {}".format(ior_cmd))

        process = subprocess.Popen(ior_cmd, stdout=subprocess.PIPE, shell=True)
        while True:
            output = process.stdout.readline()
            if output == '' and process.poll() is not None:
                break
            if output and display_output:
                print output.strip()
        if process.poll() != 0:
            raise IorFailed("IOR Run process Failed with non zero exit code:{}"
                            .format(process.poll()))

    except (OSError, ValueError) as e:
        print "<IorRunFailed> Exception occurred: {0}".format(str(e))
        raise IorFailed("IOR Run process Failed")

def check_output(stdout, comp_value_write, comp_value_read, deviation):
    """ 
    Check ior output with expected values
        stdout               --stdout for avocado where ior output is stored
        comp_value_write     --Write comparison value
        comp_value_read      --Read comparison value
        deviation            --Percentage of deviation allowed
    """

    deviation_percentage = float(deviation)/100
    searchfile = open(stdout, "r")
    data = []

    try:
        # obtaining mean wr/rd values from ior output
        for num, line in enumerate(searchfile, 1):
    	    if line.startswith("Summary of all tests:"):
                if line not in data:
                    data.append(line.strip())
                for i in range(num, num+4):
                    line = (next(itertools.islice(searchfile, i))).strip()
                    data.append(line)

        mean_write_bandwidth = int(float(data[2].split()[3]))
        mean_read_bandwidth = int(float(data[3].split()[3]))

        # gathering lower and upper bounds
        low_range_value_write = int(comp_value_write -
                                   (comp_value_write * deviation_percentage))
        upper_range_value_write = int(comp_value_write +
                                     (comp_value_write * deviation_percentage))
        low_range_value_read = int(comp_value_read -
                                  (comp_value_read * deviation_percentage))
        upper_range_value_read = int(comp_value_read +
                                    (comp_value_read * deviation_percentage))

        # checking mean wr/rd values against expected values
        if (mean_write_bandwidth < low_range_value_write or
            mean_read_bandwidth < low_range_value_read):
            if (mean_write_bandwidth < low_range_value_write and
	        mean_read_bandwidth < low_range_value_read):
	        bandwidth_diff_write = comp_value_write - mean_write_bandwidth
	        bandwidth_diff_read = comp_value_read - mean_read_bandwidth
	        raise IorFailed("Mean Bandwidth for both read and write is"
                                + " below {}% range of Base Comparison Value\n".
                                format(deviation)
                                + "Mean write Bandwidth,"
                                + " {}% less than base value\n".
                                format(((float(bandwidth_diff_write)/
                                         float(comp_value_write)) * 100))
                                + "Mean read Bandwidth,"
                                + " {}% less than base value\n".
                                format(((float(bandwidth_diff_read)/
                                         float(comp_value_read)) * 100)))
	    elif mean_write_bandwidth < low_range_value_write:
	        bandwidth_diff = comp_value_write - mean_write_bandwidth
	        raise IorFailed("Mean write Bandwidth," 
                                + " {}% less than base value".
                                format(((float(bandwidth_diff)/
                                         float(comp_value_write)) * 100)))
	    else:
	        bandwidth_diff = comp_value_read - mean_read_bandwidth
	        raise IorFailed("Mean read Bandwidth,"
                                + " {}% less than base value".
		                format(((float(bandwidth_diff)/
                                         float(comp_value_read)) * 100)))

        if ((mean_write_bandwidth in range(low_range_value_write,
             upper_range_value_write)) and (mean_read_bandwidth in 
             range(low_range_value_read, upper_range_value_read))):
            return "PASS"
        if ((mean_write_bandwidth > upper_range_value_write) or
           (mean_read_bandwidth > upper_range_value_read)):
	    if ((mean_write_bandwidth > upper_range_value_write) and
	       (mean_read_bandwidth > upper_range_value_read)):
	        bandwidth_diff_write = mean_write_bandwidth - comp_value_write
	        bandwidth_diff_read = mean_read_bandwidth - comp_value_read
	        print ("Mean write Bandwidth, {}% more than base value".
		       format(((float(bandwidth_diff_write)/
                                float(comp_value_write)) * 100)))
	        print ("Mean read Bandwidth, {}% more than base value".
		       format(((float(bandwidth_diff_read)/
                                float(comp_value_read)) * 100)))
                return "PASS"
	    elif mean_write_bandwidth > upper_range_value_write:
	        bandwidth_diff = mean_write_bandwidth - comp_value_write
	        print ("Mean write Bandwidth, {}% more than base value".
		       format(((float(bandwidth_diff)/
                                float(comp_value_write)) * 100)))
                return "PASS"
	    else:
	        bandwidth_diff = mean_read_bandwidth - comp_value_read
	        print ("Mean read Bandwidth, {}% more than base value".
		       format(((float(bandwidth_diff)/
                                float(comp_value_read)) * 100)))
                return "PASS"
    except (StopIteration) as error:
        print error

# Enable this whenever needs to check
# if the script is functioning normally.
#if __name__ == "__main__":
#    IorBuild()
