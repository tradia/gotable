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

#ifndef _GO_TABLE_H_
#define _GO_TABLE_H_

#include <stdint.h>
#include <stdlib.h>
#include <string>
#include <set>
#include <vector>

namespace gotable {

using std::string;
using std::vector;

// GoTable Error Code List
enum {
	EcNotExist    = 1,   // Key NOT exist
	EcOk          = 0,   // Success
	EcCasNotMatch = -1,  // CAS not match, get new CAS and try again
	EcTempFail    = -2,  // Temporary failed, retry may fix this
	EcUnknownCmd  = -10, // Unknown cmd
	EcAuthFailed  = -11, // Authorize failed
	EcNoPrivilege = -12, // No access privilege
	EcWriteSlave  = -13, // Can NOT write slave directly
	EcSlaveCas    = -14, // Invalid CAS on slave for GET/MGET
	EcReadFail    = -15, // Read failed
	EcWriteFail   = -16, // Write failed
	EcDecodeFail  = -17, // Decode request PKG failed
	EcInvDbId     = -18, // Invalid DB ID (cannot be 255)
	EcInvRowKey   = -19, // RowKey length should be [1 ~ 255]
	EcInvValue    = -20, // Value length should be [0 ~ 1MB]
	EcInvPkgLen   = -21, // Pkg length should be less than 2MB
	EcInvScanNum  = -22, // Scan request number out of range
	EcScanEnded   = -23, // Already scan/dump to end
};

struct GetArgs {
	uint8_t tableId;
	string  rowKey;
	string  colKey;
	uint32_t cas;

	GetArgs() : tableId(0), cas(0) {}

	GetArgs(uint8_t tableId, const string& rowKey, const string& colKey, uint32_t cas) :
			tableId(tableId), rowKey(rowKey), colKey(colKey), cas(cas) {}
};

struct GetReply {
	int8_t  errCode; // Error Code Replied
	uint8_t tableId;
	string  rowKey;
	string  colKey;
	string  value;
	int64_t score;
	uint32_t cas;

	GetReply() : errCode(0), tableId(0), score(0), cas(0) {}
};

struct SetArgs {
	uint8_t tableId;
	string  rowKey;
	string  colKey;
	string  value;
	int64_t score;
	uint32_t cas;

	SetArgs() : tableId(0), score(0), cas(0) {}

	SetArgs(uint8_t tableId, const string& rowKey, const string& colKey,
			const string& value, int64_t score, uint32_t cas) :
			tableId(tableId), rowKey(rowKey), colKey(colKey),
			value(value), score(score), cas(cas) {}
};

struct SetReply {
	int8_t  errCode; // Error Code Replied
	uint8_t tableId;
	string  rowKey;
	string  colKey;

	SetReply() : errCode(0), tableId(0) {}
};

struct IncrArgs {
	uint8_t tableId;
	string  rowKey;
	string  colKey;
	int64_t score;
	uint32_t cas;

	IncrArgs() : tableId(0), score(0), cas(0) {}

	IncrArgs(uint8_t tableId, const string& rowKey, const string& colKey,
			int64_t score, uint32_t cas) :
			tableId(tableId), rowKey(rowKey), colKey(colKey), score(score), cas(cas) {}
};

struct IncrReply {
	int8_t  errCode; // Error Code Replied
	uint8_t tableId;
	string  rowKey;
	string  colKey;
	string  value;
	int64_t score;

	IncrReply() : errCode(0), tableId(0), score(0) {}
};

typedef GetArgs DelArgs;
typedef SetReply DelReply;

struct ScanKV {
	string  colKey;
	string  value;
	int64_t score;

	ScanKV() : score(0) {}
};

struct ScanReply {
	uint8_t tableId;
	string  rowKey;
	vector<ScanKV> kvs;
	bool    end;    // true: Scan to end, stop now

	ScanReply() : tableId(0), kvs(), end(false) {}

private:
	struct ScanContext {
		bool zop;
		bool asc;          // true: Ascending  order; false: Descending  order
		bool orderByScore; // true: Score+ColKey; false: ColKey
		int num;           // Max number of scan reply records
	};
	ScanContext ctx;
	friend class Client;
};

struct DumpKV {
	uint8_t tableId;
	uint8_t colSpace;
	string  rowKey;
	string  colKey;
	string  value;
	int64_t score;

	DumpKV() : tableId(0), colSpace(0), score(0) {}
};

struct DumpReply {
	vector<DumpKV> kvs;
	bool end;    // true: Dump to end, stop now

private:
	struct DumpContext {
		bool     oneTable;     // Never change during dump
		uint8_t  tableId;      // Never change during dump
		uint16_t startUnitId;  // Never change during dump
		uint16_t endUnitId;    // Never change during dump
		uint16_t lastUnitId;   // The last unit ID tried to dump
		bool     unitStart;    // Next dump start from new UnitId
	};
	DumpContext ctx;
	friend class Client;
};

struct PkgOneOp;
struct PkgMultiOp;
struct PkgDumpResp;

class Client {
public:
	Client(int fd);
	~Client();

	// Dial connects to the address of GoTable server.
	// Upon success it returns a connection Client to GoTable server.
	// Otherwise NULL is returned.
	static Client* Dial(const char* ip, int port);

	// Close the connection.
	void close();

	// Change the selected database for the current connection.
	// Database 0 is selected by default.
	void select(uint8_t dbId);

	// Get the selected database ID for the current connection.
	uint8_t databaseId();

	// Authenticate to the server.
	// Return value <0 means failed, 0 means succeed.
	int auth(const char* password);

	// Ping the server.
	// Return value <0 means failed, 0 means succeed.
	int ping();

	// Get value&score of the key in default column space.
	// Parameter CAS is Compare-And-Swap, 2 means read data on master and
	// return a new CAS, 1 means read data on master machine but without a
	// new CAS, 0(NULL) means read data on any machine without a new CAS.
	// On cluster mode, routing to master machine is automatically, but on a
	// normal master/slave mode it should be done manually.
	// If CAS 1&2 sent to a slave machine, error will be returned.
	// Return value <0 means failed, 0 means succeed, 1 means key not exist.
	int get(uint8_t tableId, const string& rowKey, const string& colKey,
			string* value, int64_t* score, uint32_t* cas=NULL);

	// Get value&score of the key in "Z" sorted score column space.
	// Request and return parameters have the same meaning as the Get API.
	int zGet(uint8_t tableId, const string& rowKey, const string& colKey,
			string* value, int64_t* score, uint32_t* cas=NULL);

	// Set key/value in default column space. CAS is 0 for normal cases.
	// Use the CAS returned by GET if you want to "lock" the record.
	// Return value <0 means failed, 0 means succeed.
	int set(uint8_t tableId, const string& rowKey, const string& colKey,
			const string& value, int64_t score, uint32_t cas=0);

	// Set key/value in "Z" sorted score column space. CAS is 0 for normal cases.
	// Use the CAS returned by GET if you want to "lock" the record.
	// Return value <0 means failed, 0 means succeed.
	int zSet(uint8_t tableId, const string& rowKey, const string& colKey,
			const string& value, int64_t score, uint32_t cas=0);

	// Delete the key in default column space. CAS is 0 for normal cases.
	// Use the CAS returned by GET if you want to "lock" the record.
	// Return value <0 means failed, 0 means succeed.
	int del(uint8_t tableId, const string& rowKey, const string& colKey,
			uint32_t cas=0);

	// Delete the key in "Z" sorted score column space. CAS is 0 for normal cases.
	// Use the CAS returned by GET if you want to "lock" the record.
	// Return value <0 means failed, 0 means succeed.
	int zDel(uint8_t tableId, const string& rowKey, const string& colKey,
			uint32_t cas=0);

	// Increase key/score in default column space. CAS is 0 for normal cases.
	// Use the CAS returned by GET if you want to "lock" the record.
	// Return value <0 means failed, 0 means succeed.
	int incr(uint8_t tableId, const string& rowKey, const string& colKey,
			string* value, int64_t* score, uint32_t cas=0);

	// Increase key/score in "Z" sorted score column space. CAS is 0 for normal cases.
	// Use the CAS returned by GET if you want to "lock" the record.
	// Return value <0 means failed, 0 means succeed.
	int zIncr(uint8_t tableId, const string& rowKey, const string& colKey,
			string* value, int64_t* score, uint32_t cas=0);

	// Get values&scores of multiple keys in default column space.
	// Return value <0 means failed, 0 means succeed.
	int mGet(const vector<GetArgs>& args, vector<GetReply>* reply);

	// Get values&scores of multiple keys in "Z" sorted score column space.
	// Return value <0 means failed, 0 means succeed.
	int zmGet(const vector<GetArgs>& args, vector<GetReply>* reply);

	// Set multiple keys/values in default column space.
	// Return value <0 means failed, 0 means succeed.
	int mSet(const vector<SetArgs>& args, vector<SetReply>* reply);

	// Set multiple keys/values in "Z" sorted score column space.
	// Return value <0 means failed, 0 means succeed.
	int zmSet(const vector<SetArgs>& args, vector<SetReply>* reply);

	// Delete multiple keys in default column space.
	// Return value <0 means failed, 0 means succeed.
	int mDel(const vector<DelArgs>& args, vector<DelReply>* reply);

	// Delete multiple keys in "Z" sorted score column space.
	// Return value <0 means failed, 0 means succeed.
	int zmDel(const vector<DelArgs>& args, vector<DelReply>* reply);

	// Increase multiple keys/scores in default column space.
	// Return value <0 means failed, 0 means succeed.
	int mIncr(const vector<IncrArgs>& args, vector<IncrReply>* reply);

	// Increase multiple keys/scores in "Z" sorted score column space.
	// Return value <0 means failed, 0 means succeed.
	int zmIncr(const vector<IncrArgs>& args, vector<IncrReply>* reply);

	// Scan columns of rowKey in default column space from MIN/MAX colKey.
	// If asc is true SCAN start from the MIN colKey, else SCAN from the MAX colKey.
	// It replies at most num records.
	// Return value <0 means failed, 0 means succeed.
	int scan(uint8_t tableId, const string& rowKey,
			bool asc, int num, ScanReply* reply);

	// Scan columns of rowKey in default column space from pivot record.
	// The colKey is the pivot record where scan starts.
	// If asc is true SCAN in ASC order, else SCAN in DESC order.
	// It replies at most num records. The pivot record is excluded from the reply.
	// Return value <0 means failed, 0 means succeed.
	int scanPivot(uint8_t tableId, const string& rowKey, const string& colKey,
			bool asc, int num, ScanReply* reply);

	// Scan columns of rowKey in "Z" sorted score space from MIN/MAX colKey and score.
	// If asc is true ZSCAN start from the MIN colKey and score,
	// else ZSCAN from the MAX colKey and score.
	// If orderByScore is true ZSCAN order by score+colKey, else ZSCAN order by colKey.
	// It replies at most num records.
	// Return value <0 means failed, 0 means succeed.
	int zScan(uint8_t tableId, const string& rowKey,
			bool asc, bool orderByScore, int num, ScanReply* reply);

	// Scan columns of rowKey in "Z" sorted score space from pivot record.
	// The colKey and score is the pivot record where scan starts.
	// If asc is true ZSCAN in ASC order, else ZSCAN in DESC order.
	// If orderByScore is true ZSCAN order by score+colKey, else ZSCAN order by colKey.
	// It replies at most num records. The pivot record is excluded from the reply.
	// Return value <0 means failed, 0 means succeed.
	int zScanPivot(uint8_t tableId, const string& rowKey, const string& colKey, int64_t score,
			bool asc, bool orderByScore, int num, ScanReply* reply);

	// Scan/ZScan more records.
	// Return value <0 means failed, 0 means succeed.
	int scanMore(const ScanReply& last, ScanReply* reply);

	// Dump records from the pivot record.
	// If oneTable is true, only dump the selected table.
	// If oneTable is false, dump all tables in current DB(dbId).
	// The pivot record is excluded from the reply.
	// Return value <0 means failed, 0 means succeed.
	int dumpPivot(bool oneTable, uint8_t tableId, uint8_t colSpace,
			const string& rowKey, const string& colKey, int64_t score,
			uint16_t startUnitId, uint16_t endUnitId, DumpReply* reply);

	// Dump all tables in current DB(database selected).
	// Return value <0 means failed, 0 means succeed.
	int dumpDB(DumpReply* reply);

	// Dump the selected Table.
	// Return value <0 means failed, 0 means succeed.
	int dumpTable(uint8_t tableId, DumpReply* reply);

	// Dump more records.
	// Return value <0 means failed, 0 means succeed.
	int dumpMore(const DumpReply& last, DumpReply* reply);

private:
	int doOneOp(bool zop, uint8_t cmd, uint8_t tableId,
			const string& rowKey, const string& colKey,
			const string& value, int64_t score, uint32_t cas,
			PkgOneOp* reply, string& pkg);

	template <typename T>
	int doMultiOp(bool zop, uint8_t cmd, const vector<T>& args,
			PkgMultiOp* reply, string& pkg);

	int doScan(bool zop, uint8_t tableId, const string& rowKey, const string& colKey,
			int64_t score, bool start, bool asc, bool orderByScore, int num,
			ScanReply* reply, PkgMultiOp* resp, string& pkg);

	int doDump(bool oneTable, uint8_t tableId, uint8_t colSpace,
			const string& rowKey, const string& colKey, int64_t score,
			uint16_t startUnitId, uint16_t endUnitId,
			DumpReply* reply, PkgDumpResp* resp, string& pkg);

private:  //disable
	Client(const Client&);
	void operator=(const Client&);

private:
	bool     closed;
	int      fd;
	uint8_t  dbId;
	uint64_t seq;
	bool              authAdmin;
	std::set<uint8_t> setAuth;
	char     buf[4096];
};

}  // namespace gotable
#endif
