//
// (C) Copyright 2019 Intel Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// GOVERNMENT LICENSE RIGHTS-OPEN SOURCE SOFTWARE
// The Government's rights to use, modify, reproduce, release, perform, display,
// or disclose this software are subject to the terms of the Apache License as
// provided in Contract No. 8F-30005.
// Any reproduction of computer software, computer software documentation, or
// portions thereof marked with this legend must also reproduce the markings.
//

package main

import (
	"github.com/daos-stack/daos/src/control/client"
	"github.com/daos-stack/daos/src/control/common"
	pb "github.com/daos-stack/daos/src/control/common/proto/mgmt"
	types "github.com/daos-stack/daos/src/control/common/storage"
	"github.com/daos-stack/daos/src/control/logging"
)

// storageCmd is the struct representing the top-level storage subcommand.
type storageCmd struct {
	Prepare storagePrepareCmd `command:"prepare" alias:"p" description:"Prepare SCM and NVMe storage attached to remote servers."`
	Scan    storageScanCmd    `command:"scan" alias:"s" description:"Scan SCM and NVMe storage attached to remote servers."`
	Format  storageFormatCmd  `command:"format" alias:"f" description:"Format SCM and NVMe storage attached to remote servers."`
	Update  storageUpdateCmd  `command:"fwupdate" alias:"u" description:"Update firmware on NVMe storage attached to remote servers."`
	Query   storageQueryCmd   `command:"query" alias:"q" description:"Query storage commands, including raw NVMe SSD device health stats and internal blobstore health info."`
}

// storagePrepareCmd is the struct representing the prep storage subcommand.
type storagePrepareCmd struct {
	logCmd
	connectedCmd
	types.StoragePrepareCmd
}

// Execute is run when storagePrepareCmd activates
func (cmd *storagePrepareCmd) Execute(args []string) error {
	var nReq *pb.PrepareNvmeReq
	var sReq *pb.PrepareScmReq

	prepNvme, prepScm, err := cmd.Validate()
	if err != nil {
		return err
	}

	if prepNvme {
		nReq = &pb.PrepareNvmeReq{
			Pciwhitelist: cmd.PCIWhiteList,
			Nrhugepages:  int32(cmd.NrHugepages),
			Targetuser:   cmd.TargetUser,
			Reset_:       cmd.Reset,
		}
	}

	if prepScm {
		if err := cmd.Warn(cmd.log); err != nil {
			return err
		}

		sReq = &pb.PrepareScmReq{Reset_: cmd.Reset}
	}

	cmd.log.Infof("NVMe & SCM preparation:\n%s",
		cmd.conns.StoragePrepare(&pb.StoragePrepareReq{Nvme: nReq, Scm: sReq}))

	return nil
}

// storageScanCmd is the struct representing the scan storage subcommand.
type storageScanCmd struct {
	logCmd
	connectedCmd
}

// run NVMe and SCM storage and health query on all connected servers
func storageScan(log logging.Logger, conns client.Connect) {
	cCtrlrs, cModules, cPmems := conns.StorageScan()
	log.Infof("NVMe SSD controllers and constituent namespaces:\n%s", cCtrlrs)
	log.Infof("SCM modules:\n%s", cModules)
	log.Infof("PMEM device files:\n%s", cPmems)
}

// Execute is run when storageScanCmd activates
func (s *storageScanCmd) Execute(args []string) error {
	storageScan(s.log, s.conns)
	return nil
}

// storageFormatCmd is the struct representing the format storage subcommand.
type storageFormatCmd struct {
	logCmd
	connectedCmd
	Force bool `short:"f" long:"force" description:"Perform format without prompting for confirmation"`
}

// run NVMe and SCM storage format on all connected servers
func storageFormat(log logging.Logger, conns client.Connect, force bool) {
	log.Info(
		"This is a destructive operation and storage devices " +
			"specified in the server config file will be erased.\n" +
			"Please be patient as it may take several minutes.\n")

	if force || common.GetConsent(log) {
		log.Info("")
		cCtrlrResults, cMountResults := conns.StorageFormat()
		log.Infof("NVMe storage format results:\n%s", cCtrlrResults)
		log.Infof("SCM storage format results:\n%s", cMountResults)
	}
}

// Execute is run when storageFormatCmd activates
func (s *storageFormatCmd) Execute(args []string) error {
	storageFormat(s.log, s.conns, s.Force)
	return nil
}

// storageUpdateCmd is the struct representing the update storage subcommand.
type storageUpdateCmd struct {
	logCmd
	connectedCmd
	Force        bool   `short:"f" long:"force" description:"Perform update without prompting for confirmation"`
	NVMeModel    string `short:"m" long:"nvme-model" description:"Only update firmware on NVMe SSDs with this model name/number." required:"1"`
	NVMeStartRev string `short:"r" long:"nvme-fw-rev" description:"Only update firmware on NVMe SSDs currently running this firmware revision." required:"1"`
	NVMeFwPath   string `short:"p" long:"nvme-fw-path" description:"Update firmware on NVMe SSDs with image file at this path (path must be accessible on all servers)." required:"1"`
	NVMeFwSlot   int    `short:"s" default:"0" long:"nvme-fw-slot" description:"Update firmware on NVMe SSDs to this firmware register."`
}

// run NVMe and SCM storage update on all connected servers
func storageUpdate(log logging.Logger, conns client.Connect, req *pb.StorageUpdateReq, force bool) {
	log.Info(
		"This could be a destructive operation and storage devices " +
			"specified in the server config file will have firmware " +
			"updated. Please check this is a supported upgrade path " +
			"and be patient as it may take several minutes.\n")

	if force || common.GetConsent(log) {
		log.Info("")
		cCtrlrResults, cModuleResults := conns.StorageUpdate(req)
		log.Infof("NVMe storage update results:\n%s", cCtrlrResults)
		log.Infof("SCM storage update results:\n%s", cModuleResults)
	}
}

// Execute is run when storageUpdateCmd activates
func (u *storageUpdateCmd) Execute(args []string) error {
	// only populate nvme fwupdate params for the moment
	storageUpdate(
		u.log,
		u.conns,
		&pb.StorageUpdateReq{
			Nvme: &pb.UpdateNvmeReq{
				Model: u.NVMeModel, Startrev: u.NVMeStartRev,
				Path: u.NVMeFwPath, Slot: int32(u.NVMeFwSlot),
			},
		}, u.Force)

	return nil
}

// TODO: implement burn-in subcommand

//func getFioConfig(c *ishell.Context) (configPath string, err error) {
//	// fetch existing configuration files
//	paths, err := mgmtClient.FetchFioConfigPaths()
//	if err != nil {
//		return
//	}
//	// cut prefix to display filenames not full path
//	configChoices := functional.Map(
//		paths, func(p string) string { return filepath.Base(p) })
//	// add custom path option
//	configChoices = append(configChoices, "custom path")
//
//	choiceIdx := c.MultiChoice(
//		configChoices,
//		"Select the fio configuration you would like to run on the selected controllers.")
//
//	// if custom path selected (last index), process input
//	if choiceIdx == len(configChoices)-1 {
//		// disable the '>>>' for cleaner same line input.
//		c.ShowPrompt(false)
//		defer c.ShowPrompt(true) // revert after user input.
//
//		c.Print("Please enter fio configuration file-path [has file extension .fio]: ")
//		path := c.ReadLine()
//
//		if path == "" {
//			return "", fmt.Errorf("no filepath provided")
//		}
//		if filepath.Ext(path) != ".fio" {
//			return "", fmt.Errorf("provided filepath does not end in .fio")
//		}
//		if !filepath.IsAbs(path) {
//			return "", fmt.Errorf("provided filepath is not absolute")
//		}
//
//		configPath = path
//	} else {
//		configPath = paths[choiceIdx]
//	}
//
//	return
//}

//func nvmeTaskLookup(
//	c *ishell.Context, ctrlrs []*pb.NvmeController, feature string) error {
//
//	switch feature {
//	case "nvme-fw-update":
//		params, err := getUpdateParams(c)
//		if err != nil {
//			c.Println("Problem reading user inputs: ", err)
//			return err
//		}
//
//		for _, ctrlr := range ctrlrs {
//			c.Printf("\nController: %+v\n", ctrlr)
//			c.Printf(
//				"\t- Updating firmware on slot %d with image %s.\n",
//				params.Slot, params.Path)
//
//			params.Ctrlr = ctrlr
//
//			newFwrev, err := mgmtClient.UpdateNvmeCtrlr(params)
//			if err != nil {
//				c.Println("\nProblem updating firmware: ", err)
//				return err
//			}
//			c.Printf(
//				"\nSuccessfully updated firmware from revision %s to %s!\n",
//				params.Ctrlr.Fwrev, newFwrev)
//		}
//	case "nvme-burn-in":
//		configPath, err := getFioConfig(c)
//		if err != nil {
//			c.Println("Problem reading user inputs: ", err)
//			return err
//		}
//
//		for _, ctrlr := range ctrlrs {
//			c.Printf("\nController: %+v\n", ctrlr)
//			c.Printf(
//				"\t- Running burn-in validation with spdk fio plugin using job file %s.\n\n",
//				filepath.Base(configPath))
//			_, err := mgmtClient.BurnInNvme(ctrlr.Id, configPath)
//			if err != nil {
//				return err
//			}
//		}
//	default:
//		c.Printf("Sorry, task '%s' has not been implemented.\n", feature)
//	}
//
//	return nil
//}
