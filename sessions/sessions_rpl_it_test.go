// +build integration

/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/
package sessions

import (
	"net/rpc"
	"net/rpc/jsonrpc"
	"path"
	"testing"
	"time"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/utils"
	"github.com/cgrates/rpcclient"
)

var smgRplcMasterCfgPath, smgRplcSlaveCfgPath string
var smgRplcMasterCfg, smgRplcSlaveCfg *config.CGRConfig
var smgRplcMstrRPC, smgRplcSlvRPC *rpc.Client

func TestSessionSRplInitCfg(t *testing.T) {
	smgRplcMasterCfgPath = path.Join(*dataDir, "conf", "samples", "smgreplcmaster")
	if smgRplcMasterCfg, err = config.NewCGRConfigFromFolder(smgRplcMasterCfgPath); err != nil {
		t.Fatal(err)
	}
	smgRplcMasterCfg.DataFolderPath = *dataDir // Share DataFolderPath through config towards StoreDb for Flush()
	config.SetCgrConfig(smgRplcMasterCfg)
	smgRplcSlaveCfgPath = path.Join(*dataDir, "conf", "samples", "smgreplcslave")
	if smgRplcSlaveCfg, err = config.NewCGRConfigFromFolder(smgRplcSlaveCfgPath); err != nil {
		t.Fatal(err)
	}
}

// Remove data in both rating and accounting db
func TestSessionSRplResetDB(t *testing.T) {
	if err := engine.InitDataDb(smgRplcMasterCfg); err != nil {
		t.Fatal(err)
	}
	if err := engine.InitStorDb(smgRplcMasterCfg); err != nil {
		t.Fatal(err)
	}
}

// Start CGR Engine
func TestSessionSRplStartEngine(t *testing.T) {
	if _, err := engine.StopStartEngine(smgRplcSlaveCfgPath, *waitRater); err != nil { // Start slave before master
		t.Fatal(err)
	}
	if _, err := engine.StartEngine(smgRplcMasterCfgPath, *waitRater); err != nil {
		t.Fatal(err)
	}
}

// Connect rpc client to rater
func TestSessionSRplApierRpcConn(t *testing.T) {
	if smgRplcMstrRPC, err = jsonrpc.Dial("tcp", smgRplcMasterCfg.ListenCfg().RPCJSONListen); err != nil {
		t.Fatal(err)
	}
	if smgRplcSlvRPC, err = jsonrpc.Dial("tcp", smgRplcSlaveCfg.ListenCfg().RPCJSONListen); err != nil {
		t.Fatal(err)
	}
}

// Load the tariff plan, creating accounts and their balances
func TestSessionSRplTPFromFolder(t *testing.T) {
	attrs := &utils.AttrLoadTpFromFolder{FolderPath: path.Join(*dataDir, "tariffplans", "oldtutorial")}
	var loadInst utils.LoadInstance
	if err := smgRplcMstrRPC.Call("ApierV2.LoadTariffPlanFromFolder", attrs, &loadInst); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Give time for scheduler to execute topups
}

func TestSessionSRplInitiate(t *testing.T) {
	var pSessions []*ActiveSession
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetPassiveSessions,
		nil, &pSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err)
	}
	args := &V1TerminateSessionArgs{
		TerminateSession: true,
		CGREvent: utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSSv1ItTerminateSession",
			Event: map[string]interface{}{
				utils.EVENT_NAME:  "TEST_EVENT",
				utils.Tenant:      "cgrates.org",
				utils.OriginID:    "123451",
				utils.ToR:         utils.VOICE,
				utils.RequestType: utils.META_PREPAID,
				utils.Account:     "1001",
				utils.Subject:     "1001",
				utils.Destination: "1004",
				utils.Category:    "call",
				utils.SetupTime:   time.Date(2018, time.January, 7, 16, 60, 0, 0, time.UTC),
				utils.AnswerTime:  time.Date(2018, time.January, 7, 16, 60, 10, 0, time.UTC),
				utils.Usage:       1*time.Minute + 30*time.Second,
			},
		},
	}
	var reply string
	if err := smgRplcMstrRPC.Call(utils.SessionSv1TerminateSession,
		args, &reply); err == nil ||
		err.Error() != rpcclient.ErrSessionNotFound.Error() { // Update should return rpcclient.ErrSessionNotFound
		t.Error(err)
	}

	argsInit := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSSv1ItAuth",
			Event: map[string]interface{}{
				utils.EVENT_NAME:  "TEST_EVENT",
				utils.Tenant:      "cgrates.org",
				utils.OriginID:    "123451",
				utils.ToR:         utils.VOICE,
				utils.RequestType: utils.META_PREPAID,
				utils.Account:     "1001",
				utils.Subject:     "1001",
				utils.Destination: "1004",
				utils.Category:    "call",
				utils.SetupTime:   time.Date(2018, time.January, 7, 16, 60, 0, 0, time.UTC),
				utils.AnswerTime:  time.Date(2018, time.January, 7, 16, 60, 10, 0, time.UTC),
				utils.Usage:       1*time.Minute + 30*time.Second,
			},
		},
	}

	var initRpl *V1InitSessionReply
	if err := smgRplcMstrRPC.Call(utils.SessionSv1InitiateSession,
		argsInit, &initRpl); err != nil {
		t.Error(err)
	}
	if initRpl.MaxUsage != utils.DurationPointer(time.Duration(90*time.Second)) {
		t.Error("Bad max usage: ", initRpl.MaxUsage)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Wait for the sessions to be populated
	var aSessions []*ActiveSession
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetActiveSessions,
		map[string]string{utils.OriginID: "123451"}, &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 {
		t.Errorf("Unexpected number of sessions received: %+v", aSessions)
	} else if aSessions[0].Usage != time.Duration(90)*time.Second {
		t.Errorf("Received usage: %v", aSessions[0].Usage)
	}
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetPassiveSessions,
		map[string]string{utils.OriginID: "123451"}, &pSessions); err != nil {
		t.Error(err)
	} else if len(pSessions) != 1 {
		t.Errorf("PassiveSessions: %+v", pSessions)
	} else if pSessions[0].Usage != time.Duration(90*time.Second) {
		t.Errorf("PassiveSession: %+v", pSessions[0])
	}
}

// Update on slave
func TestSessionSRplUpdate(t *testing.T) {
	args := &V1UpdateSessionArgs{
		UpdateSession: true,
		CGREvent: utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSSv1ItUpdateSession",
			Event: map[string]interface{}{
				utils.EVENT_NAME:  "TEST_EVENT",
				utils.Tenant:      "cgrates.org",
				utils.OriginID:    "123451",
				utils.ToR:         utils.VOICE,
				utils.RequestType: utils.META_PREPAID,
				utils.Account:     "1001",
				utils.Subject:     "1001",
				utils.Destination: "1004",
				utils.Category:    "call",
				utils.SetupTime:   time.Date(2018, time.January, 7, 16, 60, 0, 0, time.UTC),
				utils.AnswerTime:  time.Date(2018, time.January, 7, 16, 60, 10, 0, time.UTC),
				utils.Usage:       1 * time.Minute,
			},
		},
	}

	var rply V1UpdateSessionReply
	if err := smgRplcSlvRPC.Call(utils.SessionSv1UpdateSession,
		args, &rply); err != nil {
		t.Error(err)
	} else if rply.MaxUsage != utils.DurationPointer(time.Duration(time.Minute)) {
		t.Error("Bad max usage: ", rply.MaxUsage)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Wait for the sessions to be populated
	var aSessions []*ActiveSession
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetActiveSessions,
		map[string]string{utils.OriginID: "123451"}, &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 1 {
		t.Errorf("Unexpected number of sessions received: %+v", aSessions)
	} else if aSessions[0].Usage != time.Duration(150)*time.Second {
		t.Errorf("Received usage: %v", aSessions[0].Usage)
	}
	var pSessions []*ActiveSession
	// Make sure we don't have passive session on active host
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetPassiveSessions, nil,
		&pSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err)
	}
	// Master should not longer have activeSession
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetActiveSessions,
		map[string]string{utils.OriginID: "123451"}, &aSessions); err == nil ||
		err.Error() != utils.ErrNotFound.Error() {
		t.Error(err)
	}
	cgrID := GetSetCGRID(engine.NewSafEvent(args.CGREvent.Event))
	// Make sure session was replicated
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetPassiveSessions,
		nil, &pSessions); err != nil {
		t.Error(err)
	} else if len(pSessions) != 1 {
		t.Errorf("PassiveSessions: %+v", pSessions)
	} else if pSessions[0].CGRID != cgrID {
		t.Errorf("PassiveSession: %+v", pSessions[0])
	} else if pSessions[0].Usage != time.Duration(150*time.Second) {
		t.Errorf("PassiveSession: %+v", pSessions[0])
	}
}

func TestSessionSRplTerminate(t *testing.T) {
	args := &V1TerminateSessionArgs{
		TerminateSession: true,
		CGREvent: utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSSv1ItTerminateSession",
			Event: map[string]interface{}{
				utils.EVENT_NAME:  "TEST_EVENT",
				utils.Tenant:      "cgrates.org",
				utils.OriginID:    "123451",
				utils.ToR:         utils.VOICE,
				utils.RequestType: utils.META_PREPAID,
				utils.Account:     "1001",
				utils.Subject:     "1001",
				utils.Destination: "1004",
				utils.Category:    "call",
				utils.SetupTime:   time.Date(2018, time.January, 7, 16, 60, 0, 0, time.UTC),
				utils.AnswerTime:  time.Date(2018, time.January, 7, 16, 60, 10, 0, time.UTC),
				utils.Usage:       3 * time.Minute,
			},
		},
	}
	var reply string
	if err := smgRplcMstrRPC.Call(utils.SessionSv1TerminateSession, args, &reply); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Wait for the sessions to be populated
	var aSessions []*ActiveSession
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetActiveSessions,
		map[string]string{utils.OriginID: "123451"}, &aSessions); err == nil ||
		err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetActiveSessions,
		map[string]string{utils.OriginID: "123451"}, &aSessions); err == nil ||
		err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
	var pSessions map[string][]*Session
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetPassiveSessions,
		nil, &pSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err)
	}
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetPassiveSessions,
		nil, &pSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err)
	}
}

func TestSessionSRplManualReplicate(t *testing.T) {
	masterProc, err := engine.StopStartEngine(smgRplcMasterCfgPath, *waitRater)
	if err != nil { // Kill both and start Master
		t.Fatal(err)
	}
	if smgRplcMstrRPC, err = jsonrpc.Dial("tcp", smgRplcMasterCfg.ListenCfg().RPCJSONListen); err != nil {
		t.Fatal(err)
	}

	argsInit1 := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSSv1ItAuth",
			Event: map[string]interface{}{
				utils.EVENT_NAME:  "TEST_EVENT",
				utils.Tenant:      "cgrates.org",
				utils.OriginID:    "123451",
				utils.ToR:         utils.VOICE,
				utils.RequestType: utils.META_PREPAID,
				utils.Account:     "1001",
				utils.Subject:     "1001",
				utils.Destination: "1004",
				utils.Category:    "call",
				utils.SetupTime:   time.Date(2018, time.January, 7, 16, 60, 0, 0, time.UTC),
				utils.AnswerTime:  time.Date(2018, time.January, 7, 16, 60, 10, 0, time.UTC),
				utils.Usage:       1*time.Minute + 30*time.Second,
			},
		},
	}

	argsInit2 := &V1InitSessionArgs{
		InitSession: true,
		CGREvent: utils.CGREvent{
			Tenant: "cgrates.org",
			ID:     "TestSSv1ItAuth",
			Event: map[string]interface{}{
				utils.EVENT_NAME:  "TEST_EVENT",
				utils.Tenant:      "cgrates.org",
				utils.OriginID:    "123481",
				utils.ToR:         utils.VOICE,
				utils.RequestType: utils.META_PREPAID,
				utils.Account:     "utils",
				utils.Subject:     "utils",
				utils.Destination: "1005",
				utils.Category:    "call",
				utils.SetupTime:   time.Date(2018, time.January, 7, 16, 60, 0, 0, time.UTC),
				utils.AnswerTime:  time.Date(2018, time.January, 7, 16, 60, 10, 0, time.UTC),
				utils.Usage:       1*time.Minute + 30*time.Second,
			},
		},
	}

	for _, args := range []*V1InitSessionArgs{argsInit1, argsInit2} {
		var initRpl *V1InitSessionReply
		if err := smgRplcMstrRPC.Call(utils.SessionSv1InitiateSession, args, &initRpl); err != nil {
			t.Error(err)
		}
		if initRpl.MaxUsage != utils.DurationPointer(time.Duration(90*time.Second)) {
			t.Error("Bad max usage: ", initRpl.MaxUsage)
		}
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Wait for the sessions to be populated
	var aSessions []*ActiveSession
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetActiveSessions, nil, &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 2 {
		t.Errorf("Unexpected number of sessions received: %+v", aSessions)
	} else if aSessions[0].Usage != time.Duration(90)*time.Second {
		t.Errorf("Received usage: %v", aSessions[0].Usage)
	}
	// Start slave, should not have any active session at beginning
	slave, err := engine.StartEngine(smgRplcSlaveCfgPath, *waitRater)
	if err != nil {
		t.Fatal(err)
	}
	if err := slave.Process.Kill(); err != nil { // restart the slave
		t.Error(err)
	}
	if _, err := engine.StartEngine(smgRplcSlaveCfgPath, *waitRater); err != nil {
		t.Fatal(err)
	}
	if smgRplcSlvRPC, err = jsonrpc.Dial("tcp", smgRplcSlaveCfg.ListenCfg().RPCJSONListen); err != nil {
		t.Fatal(err)
	}
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetPassiveSessions, nil, &aSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
	argsRepl := ArgsReplicateSessions{
		Connections: []*config.HaPoolConfig{
			{
				Address:     smgRplcSlaveCfg.ListenCfg().RPCJSONListen,
				Transport:   utils.MetaJSONrpc,
				Synchronous: true},
		}}
	var repply string
	if err := smgRplcMstrRPC.Call(utils.SessionSv1ReplicateSessions, argsRepl, &repply); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Wait for the sessions to be populated
	if err := smgRplcSlvRPC.Call(utils.SessionSv1GetPassiveSessions, nil, &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 2 {
		t.Errorf("Unexpected number of sessions received: %+v", aSessions)
	} else if aSessions[0].Usage != time.Duration(90)*time.Second {
		t.Errorf("Received usage: %v", aSessions[0].Usage)
	}
	// kill master
	if err := masterProc.Process.Kill(); err != nil {
		t.Errorf("Failed to kill process, error: %v", err.Error())
	}
	var status map[string]interface{}
	if err := smgRplcMstrRPC.Call("Responder.Status", "", &status); err == nil { // master should not longer be reachable
		t.Error(err, status)
	}
	if err := smgRplcSlvRPC.Call("Responder.Status", "", &status); err != nil { // slave should be still operational
		t.Error(err)
	}
	// start master
	if _, err := engine.StartEngine(smgRplcMasterCfgPath, *waitRater); err != nil {
		t.Fatal(err)
	}
	if smgRplcMstrRPC, err = jsonrpc.Dial("tcp", smgRplcMasterCfg.ListenCfg().RPCJSONListen); err != nil {
		t.Fatal(err)
	}
	// Master should have no session active/passive
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetActiveSessions, nil, &aSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetPassiveSessions, nil, &aSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
	// recover passive sessions from slave
	argsRepl = ArgsReplicateSessions{
		Passive: true,
		Connections: []*config.HaPoolConfig{
			{
				Address:     smgRplcMasterCfg.ListenCfg().RPCJSONListen,
				Transport:   utils.MetaJSONrpc,
				Synchronous: true},
		}}
	if err := smgRplcSlvRPC.Call(utils.SessionSv1ReplicateSessions, argsRepl, &repply); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Wait for the sessions to be populated
	// Master should have no session active/passive
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetActiveSessions, nil, &aSessions); err == nil || err.Error() != utils.ErrNotFound.Error() {
		t.Error(err, aSessions)
	}
	if err := smgRplcMstrRPC.Call(utils.SessionSv1GetPassiveSessions, nil, &aSessions); err != nil {
		t.Error(err)
	} else if len(aSessions) != 2 {
		t.Errorf("Unexpected number of sessions received: %+v", aSessions)
	} else if aSessions[0].Usage != time.Duration(90)*time.Second {
		t.Errorf("Received usage: %v", aSessions[0].Usage)
	}

}

func TestSessionSRplStopCgrEngine(t *testing.T) {
	if err := engine.KillEngine(1000); err != nil {
		t.Error(err)
	}
}