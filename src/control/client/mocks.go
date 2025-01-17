//
// (C) Copyright 2018-2019 Intel Corporation.
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

package client

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	. "github.com/daos-stack/daos/src/control/common"
	pb "github.com/daos-stack/daos/src/control/common/proto/mgmt"
	. "github.com/daos-stack/daos/src/control/common/storage"
	"github.com/daos-stack/daos/src/control/logging"
	"github.com/daos-stack/daos/src/control/security"
)

var (
	MockServers      = Addresses{"1.2.3.4:10000", "1.2.3.5:10001"}
	MockFeatures     = []*pb.Feature{MockFeaturePB()}
	MockCtrlrs       = NvmeControllers{MockControllerPB("E2010413")}
	MockSuccessState = pb.ResponseState{Status: pb.ResponseStatus_CTRL_SUCCESS}
	MockState        = pb.ResponseState{
		Status: pb.ResponseStatus_CTRL_ERR_APP,
		Error:  "example application error",
	}
	MockCtrlrResults = NvmeControllerResults{
		&pb.NvmeControllerResult{
			Pciaddr: "0000:81:00.0",
			State:   &MockState,
		},
	}
	MockModules       = ScmModules{MockModulePB()}
	MockModuleResults = ScmModuleResults{
		&pb.ScmModuleResult{
			Loc:   &pb.ScmModule_Location{},
			State: &MockState,
		},
	}
	MockPmemDevices  = PmemDevices{MockPmemDevicePB()}
	MockMounts       = ScmMounts{MockMountPB()}
	MockMountResults = ScmMountResults{
		&pb.ScmMountResult{
			Mntpoint: "/mnt/daos",
			State:    &MockState,
		},
	}
	MockErr = errors.New("unknown failure")
)

type mgmtCtlListFeaturesClient struct {
	grpc.ClientStream
	features      []*pb.Feature
	alreadyCalled bool
}

func (m *mgmtCtlListFeaturesClient) Recv() (*pb.Feature, error) {
	if m.alreadyCalled {
		return nil, io.EOF
	}
	m.alreadyCalled = true

	// TODO: expand to return multiple features in stream
	return m.features[0], nil
}

type mgmtCtlStorageFormatClient struct {
	grpc.ClientStream
	ctrlrResults  NvmeControllerResults
	mountResults  ScmMountResults
	alreadyCalled bool
}

func (m *mgmtCtlStorageFormatClient) Recv() (*pb.StorageFormatResp, error) {
	if m.alreadyCalled {
		return nil, io.EOF
	}
	m.alreadyCalled = true

	return &pb.StorageFormatResp{
		Crets: m.ctrlrResults,
		Mrets: m.mountResults,
	}, nil
}

type mgmtCtlStorageUpdateClient struct {
	grpc.ClientStream
	ctrlrResults  NvmeControllerResults
	moduleResults ScmModuleResults
	alreadyCalled bool
}

func (m *mgmtCtlStorageUpdateClient) Recv() (*pb.StorageUpdateResp, error) {
	if m.alreadyCalled {
		return nil, io.EOF
	}
	m.alreadyCalled = true

	return &pb.StorageUpdateResp{
		Crets: m.ctrlrResults,
		Mrets: m.moduleResults,
	}, nil
}

type mgmtCtlStorageBurnInClient struct {
	grpc.ClientStream
	ctrlrResults  NvmeControllerResults
	mountResults  ScmMountResults
	alreadyCalled bool
}

func (m *mgmtCtlStorageBurnInClient) Recv() (*pb.StorageBurnInResp, error) {
	if m.alreadyCalled {
		return nil, io.EOF
	}
	m.alreadyCalled = true

	return &pb.StorageBurnInResp{
		Crets: m.ctrlrResults,
		Mrets: m.mountResults,
	}, nil
}

type mgmtCtlFetchFioConfigPathsClient struct {
	grpc.ClientStream
	alreadyCalled bool
}

func (m *mgmtCtlFetchFioConfigPathsClient) Recv() (*pb.FilePath, error) {
	if m.alreadyCalled {
		return nil, io.EOF
	}
	m.alreadyCalled = true

	return &pb.FilePath{Path: "/tmp/fioconf.test.example"}, nil
}

type mockMgmtCtlClient struct {
	features      []*pb.Feature
	ctrlrs        NvmeControllers
	ctrlrResults  NvmeControllerResults
	modules       ScmModules
	moduleResults ScmModuleResults
	pmems         PmemDevices
	mountResults  ScmMountResults
	scanRet       error
	formatRet     error
	updateRet     error
	burninRet     error
}

func (m *mockMgmtCtlClient) ListFeatures(ctx context.Context, req *pb.EmptyReq, o ...grpc.CallOption) (pb.MgmtCtl_ListFeaturesClient, error) {
	return &mgmtCtlListFeaturesClient{features: m.features}, nil
}

func (m *mockMgmtCtlClient) StoragePrepare(ctx context.Context, req *pb.StoragePrepareReq, o ...grpc.CallOption) (*pb.StoragePrepareResp, error) {
	// return successful prepare results, state member messages
	// initialise with zero values indicating mgmt.CTRL_SUCCESS
	return &pb.StoragePrepareResp{
		Nvme: &pb.PrepareNvmeResp{
			State: &MockSuccessState,
		},
		Scm: &pb.PrepareScmResp{
			State: &MockSuccessState,
		},
	}, m.scanRet
}

func (m *mockMgmtCtlClient) StorageScan(ctx context.Context, req *pb.StorageScanReq, o ...grpc.CallOption) (*pb.StorageScanResp, error) {
	// return successful query results, state member messages
	// initialise with zero values indicating mgmt.CTRL_SUCCESS
	return &pb.StorageScanResp{
		Nvme: &pb.ScanNvmeResp{
			State:  &MockSuccessState,
			Ctrlrs: m.ctrlrs,
		},
		Scm: &pb.ScanScmResp{
			State:   &MockSuccessState,
			Modules: m.modules,
			Pmems:   m.pmems,
		},
	}, m.scanRet
}

func (m *mockMgmtCtlClient) StorageFormat(ctx context.Context, req *pb.StorageFormatReq, o ...grpc.CallOption) (pb.MgmtCtl_StorageFormatClient, error) {
	return &mgmtCtlStorageFormatClient{ctrlrResults: m.ctrlrResults, mountResults: m.mountResults}, m.formatRet
}

func (m *mockMgmtCtlClient) StorageUpdate(ctx context.Context, req *pb.StorageUpdateReq, o ...grpc.CallOption) (pb.MgmtCtl_StorageUpdateClient, error) {
	return &mgmtCtlStorageUpdateClient{ctrlrResults: m.ctrlrResults, moduleResults: m.moduleResults}, m.updateRet
}

func (m *mockMgmtCtlClient) StorageBurnIn(ctx context.Context, req *pb.StorageBurnInReq, o ...grpc.CallOption) (pb.MgmtCtl_StorageBurnInClient, error) {
	return &mgmtCtlStorageBurnInClient{ctrlrResults: m.ctrlrResults, mountResults: m.mountResults}, m.burninRet
}

func (m *mockMgmtCtlClient) FetchFioConfigPaths(ctx context.Context, req *pb.EmptyReq, o ...grpc.CallOption) (pb.MgmtCtl_FetchFioConfigPathsClient, error) {
	return &mgmtCtlFetchFioConfigPathsClient{}, nil
}

func newMockMgmtCtlClient(
	features []*pb.Feature,
	ctrlrs NvmeControllers,
	ctrlrResults NvmeControllerResults,
	modules ScmModules,
	moduleResults ScmModuleResults,
	pmems PmemDevices,
	mountResults ScmMountResults,
	scanRet error,
	formatRet error,
	updateRet error,
	burninRet error,
) pb.MgmtCtlClient {
	return &mockMgmtCtlClient{
		MockFeatures, ctrlrs, ctrlrResults, modules, moduleResults, pmems,
		mountResults, scanRet, formatRet, updateRet, burninRet,
	}
}

type mockMgmtSvcClient struct{}

func (m *mockMgmtSvcClient) PoolCreate(ctx context.Context, req *pb.PoolCreateReq, o ...grpc.CallOption) (*pb.PoolCreateResp, error) {
	// return successful pool creation results
	// initialise with zero values indicating mgmt.CTRL_SUCCESS
	return &pb.PoolCreateResp{}, nil
}

func (m *mockMgmtSvcClient) PoolDestroy(ctx context.Context, req *pb.PoolDestroyReq, o ...grpc.CallOption) (*pb.PoolDestroyResp, error) {
	// return successful pool destroy results
	// initialise with zero values indicating mgmt.CTRL_SUCCESS
	return &pb.PoolDestroyResp{}, nil
}

func (m *mockMgmtSvcClient) BioHealthQuery(
	ctx context.Context,
	req *pb.BioHealthReq,
	o ...grpc.CallOption,
) (*pb.BioHealthResp, error) {

	// return successful bio health results
	// initialise with zero values indicating mgmt.CTRL_SUCCESS
	return &pb.BioHealthResp{}, nil
}

func (m *mockMgmtSvcClient) SmdListDevs(
	ctx context.Context,
	req *pb.SmdDevReq,
	o ...grpc.CallOption,
) (*pb.SmdDevResp, error) {

	// return successful SMD device list
	// initialise with zero values indicating mgmt.CTRL_SUCCESS
	return &pb.SmdDevResp{}, nil
}

func (m *mockMgmtSvcClient) Join(ctx context.Context, req *pb.JoinReq, o ...grpc.CallOption) (*pb.JoinResp, error) {

	return &pb.JoinResp{}, nil
}

func (c *mockMgmtSvcClient) GetAttachInfo(ctx context.Context, in *pb.GetAttachInfoReq, opts ...grpc.CallOption) (*pb.GetAttachInfoResp, error) {
	return &pb.GetAttachInfoResp{}, nil
}

func (m *mockMgmtSvcClient) KillRank(ctx context.Context, req *pb.DaosRank, o ...grpc.CallOption) (*pb.DaosResp, error) {
	return &pb.DaosResp{}, nil
}

func newMockMgmtSvcClient() pb.MgmtSvcClient {
	return &mockMgmtSvcClient{}
}

// implement mock/stub behaviour for Control
type mockControl struct {
	address    string
	connState  connectivity.State
	connectRet error
	ctlClient  pb.MgmtCtlClient
	svcClient  pb.MgmtSvcClient
	log        logging.Logger
}

func (m *mockControl) connect(addr string, cfg *security.TransportConfig) error {
	if m.connectRet == nil {
		m.address = addr
	}

	return m.connectRet
}

func (m *mockControl) disconnect() error { return nil }

func (m *mockControl) connected() (connectivity.State, bool) {
	return m.connState, checkState(m.connState)
}

func (m *mockControl) getAddress() string { return m.address }

func (m *mockControl) getCtlClient() pb.MgmtCtlClient {
	return m.ctlClient
}

func (m *mockControl) getSvcClient() pb.MgmtSvcClient {
	return m.svcClient
}

func (m *mockControl) logger() logging.Logger {
	return m.log
}

func newMockControl(
	log logging.Logger,
	address string, state connectivity.State, connectRet error,
	cClient pb.MgmtCtlClient, sClient pb.MgmtSvcClient) Control {

	return &mockControl{address, state, connectRet, cClient, sClient, log}
}

type mockControllerFactory struct {
	state         connectivity.State
	features      []*pb.Feature
	ctrlrs        NvmeControllers
	ctrlrResults  NvmeControllerResults
	modules       ScmModules
	moduleResults ScmModuleResults
	pmems         PmemDevices
	mountResults  ScmMountResults
	log           logging.Logger
	// to provide error injection into Control objects
	scanRet    error
	formatRet  error
	updateRet  error
	burninRet  error
	killRet    error
	connectRet error
}

func (m *mockControllerFactory) create(address string, cfg *security.TransportConfig) (Control, error) {
	// returns controller with mock properties specified in constructor
	cClient := newMockMgmtCtlClient(
		m.features, m.ctrlrs, m.ctrlrResults,
		m.modules, m.moduleResults, m.pmems, m.mountResults,
		m.scanRet, m.formatRet, m.updateRet, m.burninRet)

	sClient := newMockMgmtSvcClient()

	controller := newMockControl(m.log, address, m.state, m.connectRet, cClient, sClient)

	err := controller.connect(address, cfg)

	return controller, err
}

func newMockConnect(
	log logging.Logger,
	state connectivity.State, features []*pb.Feature, ctrlrs NvmeControllers,
	ctrlrResults NvmeControllerResults, modules ScmModules,
	moduleResults ScmModuleResults, pmems PmemDevices, mountResults ScmMountResults,
	scanRet error, formatRet error, updateRet error, burninRet error,
	killRet error, connectRet error) Connect {

	return &connList{
		factory: &mockControllerFactory{
			state, MockFeatures, ctrlrs, ctrlrResults, modules,
			moduleResults, pmems, mountResults, log, scanRet, formatRet,
			updateRet, burninRet, killRet, connectRet,
		},
	}
}

func defaultMockConnect(log logging.Logger) Connect {
	return newMockConnect(
		log, connectivity.Ready, MockFeatures, MockCtrlrs, MockCtrlrResults, MockModules,
		MockModuleResults, MockPmemDevices, MockMountResults, nil, nil, nil, nil, nil, nil)
}

// NewClientFM provides a mock ClientFeatureMap for testing.
func NewClientFM(features []*pb.Feature, addrs Addresses) ClientFeatureMap {
	cf := make(ClientFeatureMap)
	for _, addr := range addrs {
		fMap := make(FeatureMap)
		for _, f := range features {
			fMap[f.Fname.Name] = fmt.Sprintf(
				"category %s, %s", f.Category.Category, f.Description)
		}
		cf[addr] = FeatureResult{fMap, nil}
	}
	return cf
}

// NewClientNvme provides a mock ClientCtrlrMap populated with ctrlr details
func NewClientNvme(ctrlrs NvmeControllers, addrs Addresses) ClientCtrlrMap {
	cMap := make(ClientCtrlrMap)
	for _, addr := range addrs {
		cMap[addr] = CtrlrResults{Ctrlrs: ctrlrs}
	}
	return cMap
}

// NewClientNvmeResults provides a mock ClientCtrlrMap populated with controller
// operation responses
func NewClientNvmeResults(
	results []*pb.NvmeControllerResult, addrs Addresses) ClientCtrlrMap {

	cMap := make(ClientCtrlrMap)
	for _, addr := range addrs {
		cMap[addr] = CtrlrResults{Responses: results}
	}
	return cMap
}

// NewClientScm provides a mock ClientModuleMap populated with scm module details
func NewClientScm(mms ScmModules, addrs Addresses) ClientModuleMap {
	cMap := make(ClientModuleMap)
	for _, addr := range addrs {
		cMap[addr] = ModuleResults{Modules: mms}
	}
	return cMap
}

// NewClientScmResults provides a mock ClientModuleMap populated with scm
// module operation responses
func NewClientScmResults(
	results []*pb.ScmModuleResult, addrs Addresses) ClientModuleMap {

	cMap := make(ClientModuleMap)
	for _, addr := range addrs {
		cMap[addr] = ModuleResults{Responses: results}
	}
	return cMap
}

// NewClientPmem provides a mock ClientPmemMap populated with pmem device file details
func NewClientPmem(pms PmemDevices, addrs Addresses) ClientPmemMap {
	cMap := make(ClientPmemMap)
	for _, addr := range addrs {
		cMap[addr] = PmemResults{Devices: pms}
	}
	return cMap
}

// NewClientScmMount provides a mock ClientMountMap populated with scm mount details
func NewClientScmMount(mounts ScmMounts, addrs Addresses) ClientMountMap {
	cMap := make(ClientMountMap)
	for _, addr := range addrs {
		cMap[addr] = MountResults{Mounts: mounts}
	}
	return cMap
}

// NewClientScmMountResults provides a mock ClientMountMap populated with scm mount
// operation responses
func NewClientScmMountResults(
	results []*pb.ScmMountResult, addrs Addresses) ClientMountMap {

	cMap := make(ClientMountMap)
	for _, addr := range addrs {
		cMap[addr] = MountResults{Responses: results}
	}
	return cMap
}
