package dictionary

func standardFields() map[int]FieldDefinition {
	return map[int]FieldDefinition{
		1:   {Tag: 1, Name: "Account", Type: "STRING", Sensitive: true},
		6:   {Tag: 6, Name: "AvgPx", Type: "PRICE"},
		8:   {Tag: 8, Name: "BeginString", Type: "STRING"},
		9:   {Tag: 9, Name: "BodyLength", Type: "LENGTH"},
		10:  {Tag: 10, Name: "CheckSum", Type: "STRING"},
		11:  {Tag: 11, Name: "ClOrdID", Type: "STRING"},
		14:  {Tag: 14, Name: "CumQty", Type: "QTY"},
		17:  {Tag: 17, Name: "ExecID", Type: "STRING"},
		21:  {Tag: 21, Name: "HandlInst", Type: "CHAR", Enums: map[string]string{"1": "AutomatedExecutionNoBrokerIntervention", "2": "AutomatedExecutionBrokerInterventionOK", "3": "ManualOrder"}},
		22:  {Tag: 22, Name: "SecurityIDSource", Type: "STRING"},
		31:  {Tag: 31, Name: "LastPx", Type: "PRICE"},
		32:  {Tag: 32, Name: "LastQty", Type: "QTY"},
		34:  {Tag: 34, Name: "MsgSeqNum", Type: "SEQNUM"},
		35:  {Tag: 35, Name: "MsgType", Type: "STRING", Enums: msgTypeEnums()},
		37:  {Tag: 37, Name: "OrderID", Type: "STRING"},
		38:  {Tag: 38, Name: "OrderQty", Type: "QTY"},
		39:  {Tag: 39, Name: "OrdStatus", Type: "CHAR", Enums: ordStatusEnums()},
		40:  {Tag: 40, Name: "OrdType", Type: "CHAR", Enums: map[string]string{"1": "Market", "2": "Limit", "3": "Stop", "4": "StopLimit"}},
		41:  {Tag: 41, Name: "OrigClOrdID", Type: "STRING"},
		44:  {Tag: 44, Name: "Price", Type: "PRICE"},
		48:  {Tag: 48, Name: "SecurityID", Type: "STRING"},
		49:  {Tag: 49, Name: "SenderCompID", Type: "STRING"},
		52:  {Tag: 52, Name: "SendingTime", Type: "UTCTIMESTAMP"},
		54:  {Tag: 54, Name: "Side", Type: "CHAR", Enums: map[string]string{"1": "Buy", "2": "Sell", "3": "BuyMinus", "4": "SellPlus", "5": "SellShort", "6": "SellShortExempt"}},
		55:  {Tag: 55, Name: "Symbol", Type: "STRING"},
		56:  {Tag: 56, Name: "TargetCompID", Type: "STRING"},
		58:  {Tag: 58, Name: "Text", Type: "STRING"},
		59:  {Tag: 59, Name: "TimeInForce", Type: "CHAR", Enums: map[string]string{"0": "Day", "1": "GoodTillCancel", "2": "AtTheOpening", "3": "ImmediateOrCancel", "4": "FillOrKill", "6": "GoodTillDate"}},
		60:  {Tag: 60, Name: "TransactTime", Type: "UTCTIMESTAMP"},
		89:  {Tag: 89, Name: "Signature", Type: "DATA", Sensitive: true},
		90:  {Tag: 90, Name: "SecureDataLen", Type: "LENGTH"},
		91:  {Tag: 91, Name: "SecureData", Type: "DATA", Sensitive: true},
		93:  {Tag: 93, Name: "SignatureLength", Type: "LENGTH"},
		96:  {Tag: 96, Name: "RawData", Type: "DATA", Sensitive: true},
		97:  {Tag: 97, Name: "PossResend", Type: "BOOLEAN", Enums: map[string]string{"N": "No", "Y": "Yes"}},
		98:  {Tag: 98, Name: "EncryptMethod", Type: "INT", Enums: map[string]string{"0": "NoneOther", "1": "PKCS", "2": "DES", "3": "PKCSDES", "4": "PGPDES", "5": "PGPDESMD5", "6": "PEMDESMD5"}},
		108: {Tag: 108, Name: "HeartBtInt", Type: "INT"},
		112: {Tag: 112, Name: "TestReqID", Type: "STRING"},
		122: {Tag: 122, Name: "OrigSendingTime", Type: "UTCTIMESTAMP"},
		141: {Tag: 141, Name: "ResetSeqNumFlag", Type: "BOOLEAN", Enums: map[string]string{"N": "No", "Y": "Yes"}},
		150: {Tag: 150, Name: "ExecType", Type: "CHAR", Enums: execTypeEnums()},
		151: {Tag: 151, Name: "LeavesQty", Type: "QTY"},
		167: {Tag: 167, Name: "SecurityType", Type: "STRING"},
		207: {Tag: 207, Name: "SecurityExchange", Type: "EXCHANGE"},
		432: {Tag: 432, Name: "ExpireDate", Type: "LOCALMKTDATE"},
		448: {Tag: 448, Name: "PartyID", Type: "STRING"},
		452: {Tag: 452, Name: "PartyRole", Type: "INT"},
		453: {Tag: 453, Name: "NoPartyIDs", Type: "NUMINGROUP"},
		553: {Tag: 553, Name: "Username", Type: "STRING", Sensitive: true},
		554: {Tag: 554, Name: "Password", Type: "STRING", Sensitive: true},
		925: {Tag: 925, Name: "NewPassword", Type: "STRING", Sensitive: true},
	}
}

func msgTypeEnums() map[string]string {
	return map[string]string{
		"0": "Heartbeat",
		"1": "TestRequest",
		"2": "ResendRequest",
		"3": "Reject",
		"4": "SequenceReset",
		"5": "Logout",
		"8": "ExecutionReport",
		"9": "OrderCancelReject",
		"A": "Logon",
		"D": "NewOrderSingle",
		"F": "OrderCancelRequest",
		"G": "OrderCancelReplaceRequest",
		"j": "BusinessMessageReject",
	}
}

func ordStatusEnums() map[string]string {
	return map[string]string{
		"0": "New",
		"1": "PartiallyFilled",
		"2": "Filled",
		"4": "Canceled",
		"6": "PendingCancel",
		"8": "Rejected",
		"A": "PendingNew",
		"E": "PendingReplace",
	}
}

func execTypeEnums() map[string]string {
	return map[string]string{
		"0": "New",
		"1": "PartialFill",
		"2": "Fill",
		"4": "Canceled",
		"5": "Replace",
		"6": "PendingCancel",
		"8": "Rejected",
		"A": "PendingNew",
		"E": "PendingReplace",
	}
}
