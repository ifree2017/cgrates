package v2

import (
	"net/rpc"
	"net/rpc/jsonrpc"
	"path"
	"reflect"
	"testing"
	"time"

	v1 "github.com/cgrates/cgrates/apier/v1"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/utils"
)

var cdrsOfflineCfgPath string
var cdrsOfflineCfg *config.CGRConfig
var cdrsOfflineRpc *rpc.Client
var cdrsOfflineConfDIR string // run the tests for specific configuration

// subtests to be executed for each confDIR
var sTestsCDRsOfflineIT = []func(t *testing.T){
	testV2CDRsOfflineInitConfig,
	testV2CDRsOfflineInitDataDb,
	testV2CDRsOfflineInitCdrDb,
	testV2CDRsOfflineStartEngine,
	testV2cdrsOfflineRpcConn,
	testV2CDRsOfflineLoadData,
	testV2CDRsOfflineBalanceUpdate,
	testV2CDRsOfflineExpiryBalance,

	testV2CDRsOfflineKillEngine,
}

// Tests starting here
func TestCDRsOfflineITMySQL(t *testing.T) {
	cdrsOfflineConfDIR = "cdrsv2mysql"
	for _, stest := range sTestsCDRsOfflineIT {
		t.Run(cdrsOfflineConfDIR, stest)
	}
}

func TestCDRsOfflineITpg(t *testing.T) {
	cdrsOfflineConfDIR = "cdrsv2psql"
	for _, stest := range sTestsCDRsOfflineIT {
		t.Run(cdrsOfflineConfDIR, stest)
	}
}

func TestCDRsOfflineITMongo(t *testing.T) {
	cdrsOfflineConfDIR = "cdrsv2mongo"
	for _, stest := range sTestsCDRsOfflineIT {
		t.Run(cdrsOfflineConfDIR, stest)
	}
}

func testV2CDRsOfflineInitConfig(t *testing.T) {
	var err error
	cdrsOfflineCfgPath = path.Join(*dataDir, "conf", "samples", cdrsOfflineConfDIR)
	if cdrsOfflineCfg, err = config.NewCGRConfigFromFolder(cdrsOfflineCfgPath); err != nil {
		t.Fatal("Got config error: ", err.Error())
	}
}

func testV2CDRsOfflineInitDataDb(t *testing.T) {
	if err := engine.InitDataDb(cdrsOfflineCfg); err != nil {
		t.Fatal(err)
	}
}

// InitDb so we can rely on count
func testV2CDRsOfflineInitCdrDb(t *testing.T) {
	if err := engine.InitStorDb(cdrsOfflineCfg); err != nil {
		t.Fatal(err)
	}
}

func testV2CDRsOfflineStartEngine(t *testing.T) {
	if _, err := engine.StopStartEngine(cdrsOfflineCfgPath, *waitRater); err != nil {
		t.Fatal(err)
	}
}

// Connect rpc client to rater
func testV2cdrsOfflineRpcConn(t *testing.T) {
	cdrsOfflineRpc, err = jsonrpc.Dial("tcp", cdrsOfflineCfg.ListenCfg().RPCJSONListen) // We connect over JSON so we can also troubleshoot if needed
	if err != nil {
		t.Fatal("Could not connect to rater: ", err.Error())
	}
}

func testV2CDRsOfflineLoadData(t *testing.T) {
	var loadInst utils.LoadInstance
	if err := cdrsOfflineRpc.Call("ApierV2.LoadTariffPlanFromFolder",
		&utils.AttrLoadTpFromFolder{FolderPath: path.Join(
			*dataDir, "tariffplans", "testit")}, &loadInst); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Duration(*waitRater) * time.Millisecond) // Give time for scheduler to execute topups
}

func testV2CDRsOfflineBalanceUpdate(t *testing.T) {

	//add a test account with balance type monetary and value 10
	attrs := &utils.AttrSetBalance{
		Tenant:      "cgrates.org",
		Account:     "test",
		BalanceType: utils.MONETARY,
		BalanceID:   utils.StringPointer(utils.META_DEFAULT),
		Value:       utils.Float64Pointer(10.0),
		Weight:      utils.Float64Pointer(10.0),
	}
	var reply string
	if err := cdrsOfflineRpc.Call("ApierV2.SetBalance", attrs, &reply); err != nil {
		t.Fatal(err)
	}
	var acnt *engine.Account
	if err := cdrsOfflineRpc.Call("ApierV2.GetAccount", &utils.AttrGetAccount{Tenant: "cgrates.org", Account: "test"}, &acnt); err != nil {
		t.Error(err)
	} else if len(acnt.BalanceMap) != 1 || acnt.BalanceMap[utils.MONETARY][0].Value != 10.0 {
		t.Errorf("Unexpected balance received: %+v", acnt.BalanceMap[utils.MONETARY][0])
	}

	var thReply *engine.ThresholdProfile
	var result string

	//create a log action
	attrsAA := &utils.AttrSetActions{ActionsId: "ACT_LOG", Actions: []*utils.TPAction{
		{Identifier: engine.LOG},
	}}
	if err := cdrsOfflineRpc.Call("ApierV2.SetActions", attrsAA, &reply); err != nil && err.Error() != utils.ErrExists.Error() {
		t.Error("Got error on ApierV2.SetActions: ", err.Error())
	} else if reply != utils.OK {
		t.Errorf("Calling ApierV2.SetActions received: %s", reply)
	}
	//make sure that the threshold don't exit
	if err := cdrsOfflineRpc.Call("ApierV1.GetThresholdProfile",
		&utils.TenantID{Tenant: "cgrates.org", ID: "THD_Test"}, &thReply); err == nil ||
		err.Error() != utils.ErrNotFound.Error() {
		t.Error(err)
	}
	//create a threshold that match out account
	tPrfl := &engine.ThresholdProfile{
		Tenant:    "cgrates.org",
		ID:        "THD_Test",
		FilterIDs: []string{"*string:Account:test"},
		MaxHits:   -1,
		MinSleep:  time.Duration(time.Second),
		Blocker:   false,
		Weight:    20.0,
		ActionIDs: []string{"ACT_LOG"},
		Async:     false,
	}
	if err := cdrsOfflineRpc.Call("ApierV1.SetThresholdProfile", tPrfl, &result); err != nil {
		t.Error(err)
	} else if result != utils.OK {
		t.Error("Unexpected reply returned", result)
	}
	if err := cdrsOfflineRpc.Call("ApierV1.GetThresholdProfile",
		&utils.TenantID{Tenant: "cgrates.org", ID: "THD_Test"}, &thReply); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(tPrfl, thReply) {
		t.Errorf("Expecting: %+v, received: %+v", tPrfl, thReply)
	}

	cgrEv := &utils.CGREvent{
		Tenant: "cgrates.org",
		Event: map[string]interface{}{
			utils.OriginID:    "testV2CDRsOfflineProcessCDR2",
			utils.OriginHost:  "192.168.1.1",
			utils.Source:      "testV2CDRsOfflineProcessCDR",
			utils.RequestType: utils.META_POSTPAID,
			utils.Category:    "call",
			utils.Account:     "test",
			utils.Subject:     "test",
			utils.Destination: "1002",
			utils.AnswerTime:  time.Date(2018, 8, 24, 16, 00, 26, 0, time.UTC),
			utils.Usage:       time.Duration(1) * time.Minute,
		},
	}
	mapEv := engine.NewMapEvent(cgrEv.Event)
	cdr, err := mapEv.AsCDR(nil, "cgrates.org", "")
	if err != nil {
		t.Error("Unexpected error received: ", err)
	}
	//process cdr should trigger balance update event
	if err := cdrsOfflineRpc.Call("CdrsV1.ProcessCDR", cdr, &reply); err != nil {
		t.Error("Unexpected error: ", err.Error())
	} else if reply != utils.OK {
		t.Error("Unexpected reply received: ", reply)
	}
	time.Sleep(time.Duration(150) * time.Millisecond) // Give time for CDR to be rated
}

func testV2CDRsOfflineExpiryBalance(t *testing.T) {

	var reply string
	acc := &utils.AttrSetActions{ActionsId: "ACT_TOPUP_TEST2", Actions: []*utils.TPAction{
		&utils.TPAction{Identifier: engine.TOPUP, BalanceType: utils.MONETARY, BalanceId: "BalanceExpired1", Units: "5",
			ExpiryTime: time.Date(2018, 8, 24, 16, 00, 26, 0, time.UTC).String(), BalanceWeight: "10", Weight: 20.0},
		&utils.TPAction{Identifier: engine.TOPUP, BalanceType: utils.MONETARY, BalanceId: "BalanceExpired2", Units: "10",
			ExpiryTime: time.Date(2018, 8, 24, 16, 00, 26, 0, time.UTC).String(), BalanceWeight: "10", Weight: 20.0},
		&utils.TPAction{Identifier: engine.TOPUP, BalanceType: utils.MONETARY, BalanceId: "NewBalance", Units: "10",
			ExpiryTime: engine.UNLIMITED, BalanceWeight: "10", Weight: 20.0},
	}}
	if err := cdrsOfflineRpc.Call("ApierV2.SetActions", acc, &reply); err != nil && err.Error() != utils.ErrExists.Error() {
		t.Error("Got error on ApierV2.SetActions: ", err.Error())
	} else if reply != utils.OK {
		t.Errorf("Calling ApierV2.SetActions received: %s", reply)
	}

	atm1 := &v1.AttrActionPlan{ActionsId: "ACT_TOPUP_TEST2", Time: "*asap", Weight: 20.0}
	atms1 := &v1.AttrSetActionPlan{Id: "AP_TEST2", ActionPlan: []*v1.AttrActionPlan{atm1}}
	if err := cdrsOfflineRpc.Call("ApierV1.SetActionPlan", atms1, &reply); err != nil {
		t.Error("Got error on ApierV1.SetActionPlan: ", err.Error())
	} else if reply != "OK" {
		t.Errorf("Calling ApierV1.SetActionPlan received: %s", reply)
	}

	if err := cdrsOfflineRpc.Call("ApierV2.SetAccount",
		&AttrSetAccount{Tenant: "cgrates.org", Account: "test2",
			ActionPlanIDs: &[]string{"AP_TEST2"}, ReloadScheduler: true},
		&reply); err != nil {
		t.Error(err)
	} else if reply != utils.OK {
		t.Errorf("Calling ApierV2.SetAccount received: %s", reply)
	}

	var acnt *engine.Account
	//verify if the third balance was added
	if err := cdrsOfflineRpc.Call("ApierV2.GetAccount",
		&utils.AttrGetAccount{Tenant: "cgrates.org", Account: "test2"}, &acnt); err != nil {
		t.Error(err)
	} else if acnt.BalanceMap[utils.MONETARY].Len() != 1 {
		t.Errorf("Unexpected balance received: %+v", utils.ToIJSON(acnt))
	}

	var thReply *engine.ThresholdProfile
	var result string

	//create a log action
	attrsA := &utils.AttrSetActions{ActionsId: "ACT_LOG", Actions: []*utils.TPAction{
		{Identifier: engine.LOG},
	}}
	if err := cdrsOfflineRpc.Call("ApierV2.SetActions", attrsA, &reply); err != nil && err.Error() != utils.ErrExists.Error() {
		t.Error("Got error on ApierV2.SetActions: ", err.Error())
	} else if reply != utils.OK {
		t.Errorf("Calling ApierV2.SetActions received: %s", reply)
	}
	//make sure that the threshold don't exit
	if err := cdrsOfflineRpc.Call("ApierV1.GetThresholdProfile",
		&utils.TenantID{Tenant: "cgrates.org", ID: "THD_Test2"}, &thReply); err == nil ||
		err.Error() != utils.ErrNotFound.Error() {
		t.Error(err)
	}
	//create a threshold that match out account
	tPrfl := &engine.ThresholdProfile{
		Tenant:    "cgrates.org",
		ID:        "THD_Test2",
		FilterIDs: []string{"*string:Account:test2"},
		ActivationInterval: &utils.ActivationInterval{
			ActivationTime: time.Date(2014, 7, 14, 14, 35, 0, 0, time.UTC),
			ExpiryTime:     time.Date(2014, 7, 14, 14, 35, 0, 0, time.UTC),
		},
		MaxHits:   -1,
		MinSleep:  time.Duration(0),
		Blocker:   false,
		Weight:    20.0,
		ActionIDs: []string{"ACT_LOG"},
		Async:     false,
	}
	if err := cdrsOfflineRpc.Call("ApierV1.SetThresholdProfile", tPrfl, &result); err != nil {
		t.Error(err)
	} else if result != utils.OK {
		t.Error("Unexpected reply returned", result)
	}
	if err := cdrsOfflineRpc.Call("ApierV1.GetThresholdProfile",
		&utils.TenantID{Tenant: "cgrates.org", ID: "THD_Test2"}, &thReply); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(tPrfl, thReply) {
		t.Errorf("Expecting: %+v, received: %+v", tPrfl, thReply)
	}

	cgrEv := &utils.CGREvent{
		Tenant: "cgrates.org",
		Event: map[string]interface{}{
			utils.OriginID:    "testV2CDRsOfflineProcessCDR1",
			utils.OriginHost:  "192.168.1.1",
			utils.Source:      "testV2CDRsOfflineProcessCDR",
			utils.RequestType: utils.META_POSTPAID,
			utils.Category:    "call",
			utils.Account:     "test2",
			utils.Subject:     "test2",
			utils.Destination: "1002",
			utils.AnswerTime:  time.Date(2018, 8, 24, 16, 00, 26, 0, time.UTC),
			utils.Usage:       time.Duration(1) * time.Minute,
		},
	}
	//process cdr should trigger balance update event
	if err := cdrsOfflineRpc.Call(utils.CdrsV2ProcessCDR, cgrEv, &reply); err != nil {
		t.Error("Unexpected error: ", err.Error())
	} else if reply != utils.OK {
		t.Error("Unexpected reply received: ", reply)
	}
	time.Sleep(time.Duration(150) * time.Millisecond) // Give time for CDR to be rated
}

func testV2CDRsOfflineKillEngine(t *testing.T) {
	if err := engine.KillEngine(*waitRater); err != nil {
		t.Error(err)
	}
}