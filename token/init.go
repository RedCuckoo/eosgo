package token

import (
	"github.com/redcuckoo/eosgo/types"
)

func init() {
	types.RegisterAction(types.AN("eosio.token"), types.ActN("transfer"), Transfer{})
	types.RegisterAction(types.AN("eosio.token"), types.ActN("issue"), Issue{})
	types.RegisterAction(types.AN("eosio.token"), types.ActN("create"), Create{})
}
