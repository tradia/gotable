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

package proto

import (
	"encoding/binary"
)

const (
	CtrlErrCode  = 0x1 // Response Error Code
	CtrlCas      = 0x2 // Compare And Switch
	CtrlColSpace = 0x4
	CtrlValue    = 0x8
	CtrlScore    = 0x10
)

const (
	ColSpaceDefault = 0 // Default column space
	ColSpaceScore1  = 1 // rowKey+score+colKey => value
	ColSpaceScore2  = 2 // rowKey+colKey => score+value
)

type KeyValue struct {
	TableId uint8
	RowKey  []byte
	ColKey  []byte
	Value   []byte // default: nil if missing
	Score   int64  // default: 0 if missing
}

// KeyValueCtrl=cCtrlFlag+cTableId+[cErrCode]+[cColSpace]
//             +cRowKeyLen+sRowKey+wColKeyLen+sColKey
//             +[dwValueLen+sValue]+[ddwScore]+[dwCas]
type KeyValueCtrl struct {
	CtrlFlag uint8
	ErrCode  uint8 // default: 0 if missing
	ColSpace uint8 // default: 0 if missing
	KeyValue
	Cas uint32 // default: 0 if missing
}

// Get, Set, Del, GetSet, GetDel, ZGet, ZSet, Sync
// PKG = HEAD+KeyValueCtrl
type PkgOneOp struct {
	PkgHead
	KeyValueCtrl
}

// MGet, MSet, MDel, MZGet, MZSet, MZDel
// PKG = HEAD+cErrCode+wNum+KeyValueCtrl[wNum]
type PkgMultiOp struct {
	ErrCode uint8 // default: 0 if missing
	PkgHead
	Kvs []KeyValueCtrl
}

// Scan, ZScan
// PKG = PkgOneOp+cDirection+cStart+wNum
type PkgScanReq struct {
	Direction uint8 // 0: Ascending  order; 1: Descending  order
	Start     uint8 // 0: Not start; 1: Start from MIN or MAX key
	Num       uint16
	PkgOneOp
}

// Scan, ZScan
// PKG = PkgMultiOp+cDirection+cStart+cEnd
type PkgScanResp struct {
	Direction uint8
	Start     uint8
	End       uint8 // 0: Not end yet; 1: Has scan to end, stop now
	PkgMultiOp
}

// Dump
// PKG = PkgOneOp+cOneTable+wStartUnitId+wEndUnitId
type PkgDumpReq struct {
	OneTable    uint8 // 0: Dump current DbId; 1: Dump only the selected TableId
	StartUnitId uint16
	EndUnitId   uint16
	PkgOneOp
}

// Dump
// PKG = PkgMultiOp+cOneTable+wStartUnitId+wEndUnitId+wLastUnitId+cEnd
type PkgDumpResp struct {
	OneTable    uint8
	StartUnitId uint16
	EndUnitId   uint16
	LastUnitId  uint16 // Last dump Unit ID
	End         uint8  // 0: Not end yet; 1: Has dump to end, stop now
	PkgMultiOp
}

func (kv *KeyValueCtrl) Length() int {
	// KeyValueCtrl=cCtrlFlag+cTableId+[cErrCode]+[cColSpace]
	//             +cRowKeyLen+sRowKey+wColKeyLen+sColKey
	//             +[dwValueLen+sValue]+[ddwScore]+[dwCas]
	var n = 2
	if kv.CtrlFlag&CtrlErrCode != 0 {
		n += 1
	}
	if kv.CtrlFlag&CtrlColSpace != 0 {
		n += 1
	}
	n += 1 + len(kv.RowKey) + 2 + len(kv.ColKey)
	if kv.CtrlFlag&CtrlValue != 0 {
		n += 4 + len(kv.Value)
	}
	if kv.CtrlFlag&CtrlScore != 0 {
		n += 8
	}
	if kv.CtrlFlag&CtrlCas != 0 {
		n += 4
	}
	return n
}

func (kv *KeyValueCtrl) Encode(pkg []byte) (int, error) {
	if len(pkg) < kv.Length() {
		return 0, ErrPkgLen
	}
	if len(kv.RowKey) > MaxUint8 {
		return 0, ErrRowKeyLen
	}
	if len(kv.ColKey) > MaxUint16 {
		return 0, ErrColKeyLen
	}
	if len(kv.Value) > MaxValueLen {
		return 0, ErrValueLen
	}

	var n int
	pkg[n] = kv.CtrlFlag
	n += 1
	pkg[n] = kv.TableId
	n += 1
	if kv.CtrlFlag&CtrlErrCode != 0 {
		pkg[n] = kv.ErrCode
		n += 1
	}
	if kv.CtrlFlag&CtrlColSpace != 0 {
		pkg[n] = kv.ColSpace
		n += 1
	}

	pkg[n] = uint8(len(kv.RowKey))
	n += 1
	copy(pkg[n:], kv.RowKey)
	n += len(kv.RowKey)

	binary.BigEndian.PutUint16(pkg[n:], uint16(len(kv.ColKey)))
	n += 2
	copy(pkg[n:], kv.ColKey)
	n += len(kv.ColKey)

	if kv.CtrlFlag&CtrlValue != 0 {
		binary.BigEndian.PutUint32(pkg[n:], uint32(len(kv.Value)))
		n += 4
		copy(pkg[n:], kv.Value)
		n += len(kv.Value)
	}
	if kv.CtrlFlag&CtrlScore != 0 {
		binary.BigEndian.PutUint64(pkg[n:], uint64(kv.Score))
		n += 8
	}
	if kv.CtrlFlag&CtrlCas != 0 {
		binary.BigEndian.PutUint32(pkg[n:], kv.Cas)
		n += 4
	}
	return n, nil
}

func (kv *KeyValueCtrl) Decode(pkg []byte) (int, error) {
	var pkgLen = len(pkg)
	var n = 0
	if n+2 > pkgLen {
		return n, ErrPkgLen
	}
	kv.CtrlFlag = pkg[n]
	n += 1
	kv.TableId = pkg[n]
	n += 1

	if kv.CtrlFlag&CtrlErrCode != 0 {
		if n+1 > pkgLen {
			return n, ErrPkgLen
		}
		kv.ErrCode = pkg[n]
		n += 1
	} else {
		kv.ErrCode = 0
	}
	if kv.CtrlFlag&CtrlColSpace != 0 {
		if n+1 > pkgLen {
			return n, ErrPkgLen
		}
		kv.ColSpace = pkg[n]
		n += 1
	} else {
		kv.ColSpace = 0
	}

	if n+1 > pkgLen {
		return n, ErrPkgLen
	}
	var rowKeyLen = int(pkg[n])
	n += 1
	if n+rowKeyLen+2 > pkgLen {
		return n, ErrPkgLen
	}
	kv.RowKey = pkg[n : n+rowKeyLen]
	n += rowKeyLen

	var colKeyLen = int(binary.BigEndian.Uint16(pkg[n:]))
	n += 2
	if n+colKeyLen > pkgLen {
		return n, ErrPkgLen
	}
	kv.ColKey = pkg[n : n+colKeyLen]
	n += colKeyLen

	if kv.CtrlFlag&CtrlValue != 0 {
		if n+4 > pkgLen {
			return n, ErrPkgLen
		}
		var valueLen = int(binary.BigEndian.Uint32(pkg[n:]))
		n += 4
		if valueLen > MaxValueLen {
			return n, ErrValueLen
		}
		if n+valueLen > pkgLen {
			return n, ErrPkgLen
		}
		kv.Value = pkg[n : n+valueLen]
		n += valueLen
	} else {
		kv.Value = nil
	}
	if kv.CtrlFlag&CtrlScore != 0 {
		if n+8 > pkgLen {
			return n, ErrPkgLen
		}
		kv.Score = int64(binary.BigEndian.Uint64(pkg[n:]))
		n += 8
	} else {
		kv.Score = 0
	}
	if kv.CtrlFlag&CtrlCas != 0 {
		if n+4 > pkgLen {
			return n, ErrPkgLen
		}
		kv.Cas = binary.BigEndian.Uint32(pkg[n:])
		n += 4
	} else {
		kv.Cas = 0
	}
	return n, nil
}

func (p *PkgOneOp) Length() int {
	// PKG = HEAD+KeyValueCtrl
	return HeadSize + p.KeyValueCtrl.Length()
}

func (p *PkgOneOp) SetErrCode(errCode uint8) {
	p.ErrCode = errCode
}

func (p *PkgOneOp) Encode(pkg []byte) (int, error) {
	n, err := p.PkgHead.Encode(pkg)
	if err != nil {
		return n, err
	}

	m, err := p.KeyValueCtrl.Encode(pkg[n:])
	if err != nil {
		return n, err
	}
	n += m

	OverWriteLen(pkg, n)
	return n, nil
}

func (p *PkgOneOp) Decode(pkg []byte) (int, error) {
	n, err := p.PkgHead.Decode(pkg)
	if err != nil {
		return 0, err
	}

	m, err := p.KeyValueCtrl.Decode(pkg[n:])
	if err != nil {
		return n, err
	}
	n += m

	return n, nil
}

func (p *PkgMultiOp) Length() int {
	// PKG = HEAD+cErrCode+wNum+KeyValueCtrl[wNum]
	var n = HeadSize + 3
	for i := 0; i < len(p.Kvs); i++ {
		n += p.Kvs[i].Length()
	}
	return n
}

func (p *PkgMultiOp) SetErrCode(errCode uint8) {
	p.ErrCode = errCode
}

func (p *PkgMultiOp) Encode(pkg []byte) (int, error) {
	var numKvs = len(p.Kvs)
	if numKvs > MaxUint16 {
		return 0, ErrKvArrayLen
	}

	n, err := p.PkgHead.Encode(pkg)
	if err != nil {
		return n, err
	}

	if n+3 > len(pkg) {
		return 0, ErrPkgLen
	}
	pkg[n] = p.ErrCode
	n += 1
	binary.BigEndian.PutUint16(pkg[n:], uint16(numKvs))
	n += 2

	for i := 0; i < numKvs; i++ {
		m, err := p.Kvs[i].Encode(pkg[n:])
		if err != nil {
			return n, err
		}
		n += m
	}

	OverWriteLen(pkg, n)
	return n, nil
}

func (p *PkgMultiOp) Decode(pkg []byte) (int, error) {
	n, err := p.PkgHead.Decode(pkg)
	if err != nil {
		return 0, err
	}

	if n+3 > len(pkg) {
		return n, ErrPkgLen
	}
	p.ErrCode = pkg[n]
	n += 1
	var numKvs = int(binary.BigEndian.Uint16(pkg[n:]))
	n += 2

	p.Kvs = make([]KeyValueCtrl, numKvs)
	for i := 0; i < numKvs; i++ {
		m, err := p.Kvs[i].Decode(pkg[n:])
		if err != nil {
			return n, err
		}
		n += m
	}

	return n, nil
}

func (p *PkgScanReq) Length() int {
	// PKG = PkgOneOp+cDirection+cStart+wNum
	return p.PkgOneOp.Length() + 4
}

func (p *PkgScanReq) Encode(pkg []byte) (int, error) {
	n, err := p.PkgOneOp.Encode(pkg)
	if err != nil {
		return n, err
	}

	if n+4 > len(pkg) {
		return 0, ErrPkgLen
	}
	pkg[n] = p.Direction
	n += 1
	pkg[n] = p.Start
	n += 1
	binary.BigEndian.PutUint16(pkg[n:], p.Num)
	n += 2

	OverWriteLen(pkg, n)
	return n, nil
}

func (p *PkgScanReq) Decode(pkg []byte) (int, error) {
	n, err := p.PkgOneOp.Decode(pkg)
	if err != nil {
		return n, err
	}

	if n+4 > len(pkg) {
		return n, ErrPkgLen
	}
	p.Direction = pkg[n]
	n += 1
	p.Start = pkg[n]
	n += 1
	p.Num = binary.BigEndian.Uint16(pkg[n:])
	n += 2

	return n, nil
}

func (p *PkgScanResp) Length() int {
	// PKG = PkgMultiOp+cDirection+cStart+cEnd
	return p.PkgMultiOp.Length() + 3
}

func (p *PkgScanResp) Encode(pkg []byte) (int, error) {
	n, err := p.PkgMultiOp.Encode(pkg)
	if err != nil {
		return n, err
	}

	if n+3 > len(pkg) {
		return n, ErrPkgLen
	}
	pkg[n] = p.Direction
	n += 1
	pkg[n] = p.Start
	n += 1
	pkg[n] = p.End
	n += 1

	OverWriteLen(pkg, n)
	return n, nil
}

func (p *PkgScanResp) Decode(pkg []byte) (int, error) {
	n, err := p.PkgMultiOp.Decode(pkg)
	if err != nil {
		return n, err
	}

	if n+3 > len(pkg) {
		return n, ErrPkgLen
	}
	p.Direction = pkg[n]
	n += 1
	p.Start = pkg[n]
	n += 1
	p.End = pkg[n]
	n += 1

	return n, nil
}

func (p *PkgDumpReq) Length() int {
	// PKG = PkgOneOp+cOneTable+wStartUnitId+wEndUnitId
	return p.PkgOneOp.Length() + 5
}

func (p *PkgDumpReq) Encode(pkg []byte) (int, error) {
	n, err := p.PkgOneOp.Encode(pkg)
	if err != nil {
		return n, err
	}

	if n+5 > len(pkg) {
		return n, ErrPkgLen
	}
	pkg[n] = p.OneTable
	n += 1
	binary.BigEndian.PutUint16(pkg[n:], p.StartUnitId)
	n += 2
	binary.BigEndian.PutUint16(pkg[n:], p.EndUnitId)
	n += 2

	OverWriteLen(pkg, n)
	return n, nil
}

func (p *PkgDumpReq) Decode(pkg []byte) (int, error) {
	n, err := p.PkgOneOp.Decode(pkg)
	if err != nil {
		return n, err
	}

	if n+5 > len(pkg) {
		return n, ErrPkgLen
	}
	p.OneTable = pkg[n]
	n += 1
	p.StartUnitId = binary.BigEndian.Uint16(pkg[n:])
	n += 2
	p.EndUnitId = binary.BigEndian.Uint16(pkg[n:])
	n += 2

	return n, nil
}

func (p *PkgDumpResp) Length() int {
	// PKG = PkgMultiOp+cOneTable+wStartUnitId+wEndUnitId+wLastUnitId+cEnd
	return p.PkgMultiOp.Length() + 8
}

func (p *PkgDumpResp) Encode(pkg []byte) (int, error) {
	n, err := p.PkgMultiOp.Encode(pkg)
	if err != nil {
		return n, err
	}

	if n+8 > len(pkg) {
		return n, ErrPkgLen
	}
	pkg[n] = p.OneTable
	n += 1
	binary.BigEndian.PutUint16(pkg[n:], p.StartUnitId)
	n += 2
	binary.BigEndian.PutUint16(pkg[n:], p.EndUnitId)
	n += 2
	binary.BigEndian.PutUint16(pkg[n:], p.LastUnitId)
	n += 2
	pkg[n] = p.End
	n += 1

	OverWriteLen(pkg, n)
	return n, nil
}

func (p *PkgDumpResp) Decode(pkg []byte) (int, error) {
	n, err := p.PkgMultiOp.Decode(pkg)
	if err != nil {
		return n, err
	}

	if n+8 > len(pkg) {
		return n, ErrPkgLen
	}
	p.OneTable = pkg[n]
	n += 1
	p.StartUnitId = binary.BigEndian.Uint16(pkg[n:])
	n += 2
	p.EndUnitId = binary.BigEndian.Uint16(pkg[n:])
	n += 2
	p.LastUnitId = binary.BigEndian.Uint16(pkg[n:])
	n += 2
	p.End = pkg[n]
	n += 1

	return n, nil
}
