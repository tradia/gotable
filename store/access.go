// Copyright 2015 stevejiang. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import (
	"github.com/stevejiang/gotable/config"
	"github.com/stevejiang/gotable/ctrl"
)

type WriteAccess struct {
	replication bool // Replication slaver
	hasMaster   bool
	migration   bool
	unitId      uint16
}

func NewWriteAccess(replication bool, mc *config.MasterConfig) *WriteAccess {
	hasMaster, migration, unitId := mc.GetMasterUnit()
	return &WriteAccess{replication, hasMaster, migration, unitId}
}

// Do we have right to write this key?
func (m *WriteAccess) CheckKey(dbId, tableId uint8, rowKey []byte) bool {
	if m.replication {
		return true // Accept all replication data
	}

	if !m.hasMaster {
		return true
	}

	if m.migration {
		return m.unitId != ctrl.GetUnitId(dbId, tableId, rowKey)
	} else {
		return false
	}
}

// Do we have right to write this unit?
func (m *WriteAccess) CheckUnit(unitId uint16) bool {
	if m.replication {
		return true // Accept all replication data
	}

	if !m.hasMaster {
		return true
	}

	if m.migration {
		return m.unitId != unitId
	} else {
		return false
	}
}

// return false: no right to write!
// return true: may have right to write, need to check key or unit again
func (m *WriteAccess) Check() bool {
	if m.replication {
		return true // Accept all replication data
	}

	if m.hasMaster && !m.migration {
		return false
	}

	return true
}
