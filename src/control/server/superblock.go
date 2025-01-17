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

package server

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"

	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/server/ioserver"
)

const (
	defaultStoragePath = "/mnt/daos"
	defaultGroupName   = "daos_io_server"
	superblockVersion  = 0
)

// Superblock is the per-Instance superblock
type Superblock struct {
	Version     uint8
	UUID        string
	System      string
	Rank        *ioserver.Rank
	ValidRank   bool
	MS          bool
	CreateMS    bool
	BootstrapMS bool
}

// TODO: Marshal/Unmarshal using a binary representation?

// Marshal transforms the Superblock into a storable representation
func (sb *Superblock) Marshal() ([]byte, error) {
	data, err := yaml.Marshal(sb)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to marshal %+v", sb)
	}
	return data, nil
}

// Unmarshal reconstitutes a Superblock from a Marshaled representation
func (sb *Superblock) Unmarshal(raw []byte) error {
	return yaml.Unmarshal(raw, sb)
}

func (srv *IOServerInstance) superblockPath() string {
	storagePath := srv.runner.Config.Storage.SCM.MountPoint
	if storagePath == "" {
		storagePath = defaultStoragePath
	}
	return filepath.Join(srv.fsRoot, storagePath, "superblock")
}

func (srv *IOServerInstance) setSuperblock(sb *Superblock) {
	srv.Lock()
	defer srv.Unlock()
	srv._superblock = sb
}

func (srv *IOServerInstance) getSuperblock() *Superblock {
	srv.RLock()
	defer srv.RUnlock()
	return srv._superblock
}

func (srv *IOServerInstance) hasSuperblock() bool {
	return srv.getSuperblock() != nil
}

// NeedsSuperblock indicates whether or not the instance appears
// to need a superblock to be created in order to start.
func (srv *IOServerInstance) NeedsSuperblock() (bool, error) {
	if srv.hasSuperblock() {
		return false, nil
	}

	scmCfg, err := srv.scmConfig()
	if err != nil {
		return false, err
	}
	srv.log.Debugf("%s: checking superblock", scmCfg.MountPoint)

	err = srv.ReadSuperblock()
	if os.IsNotExist(errors.Cause(err)) {
		srv.log.Debugf("%s: needs superblock (doesn't exist)", scmCfg.MountPoint)
		return true, nil
	}

	if err != nil {
		return true, errors.Wrap(err, "failed to read existing superblock")
	}

	return false, nil
}

// CreateSuperblock creates the superblock for this instance.
func (srv *IOServerInstance) CreateSuperblock(msInfo *mgmtInfo) error {
	if err := srv.MountScmDevice(); err != nil {
		return err
	}

	u, err := uuid.NewV4()
	if err != nil {
		return errors.Wrap(err, "Failed to generate instance UUID")
	}

	systemName := srv.runner.Config.SystemName
	if systemName == "" {
		systemName = defaultGroupName
	}

	superblock := &Superblock{
		Version:     superblockVersion,
		UUID:        u.String(),
		System:      systemName,
		ValidRank:   msInfo.isReplica && msInfo.shouldBootstrap,
		MS:          msInfo.isReplica,
		CreateMS:    msInfo.isReplica,
		BootstrapMS: msInfo.shouldBootstrap,
	}

	if srv.runner.Config.Rank != nil || msInfo.isReplica && msInfo.shouldBootstrap {
		superblock.Rank = new(ioserver.Rank)
		if srv.runner.Config.Rank != nil {
			*superblock.Rank = *srv.runner.Config.Rank
		}
	}
	srv.setSuperblock(superblock)

	return srv.WriteSuperblock()
}

// WriteSuperblock writes the instance's superblock
// to storage.
func (srv *IOServerInstance) WriteSuperblock() error {
	return WriteSuperblock(srv.superblockPath(), srv.getSuperblock())
}

// ReadSuperblock reads the instance's superblock
// from storage.
func (srv *IOServerInstance) ReadSuperblock() error {
	needsFormat, err := srv.NeedsScmFormat()
	if err != nil {
		return errors.Wrap(err, "failed to check storage formatting")
	}
	if needsFormat {
		return errors.New("can't read superblock from unformatted storage")
	}

	if err := srv.MountScmDevice(); err != nil {
		return errors.Wrap(err, "failed to mount SCM device")
	}

	sb, err := ReadSuperblock(srv.superblockPath())
	if err != nil {
		return errors.Wrap(err, "failed to read instance superblock")
	}
	srv.setSuperblock(sb)

	return nil
}

// WriteSuperblock writes a Superblock to storage.
func WriteSuperblock(sbPath string, sb *Superblock) error {
	data, err := sb.Marshal()
	if err != nil {
		return err
	}

	return errors.Wrapf(common.WriteFileAtomic(sbPath, data, 0600),
		"Failed to write Superblock to %s", sbPath)
}

// ReadSuperblock reads a Superblock from storage.
func ReadSuperblock(sbPath string) (*Superblock, error) {
	data, err := ioutil.ReadFile(sbPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read Superblock from %s", sbPath)
	}

	sb := &Superblock{}
	if err := sb.Unmarshal(data); err != nil {
		return nil, err
	}

	return sb, nil
}
