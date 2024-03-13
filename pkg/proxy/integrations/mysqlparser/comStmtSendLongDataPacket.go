package mysqlparser

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

type COM_STMT_SEND_LONG_DATA struct {
	StatementID uint32 `json:"statement_id,omitempty" yaml:"statement_id,omitempty,flow"`
	ParameterID uint16 `json:"parameter_id,omitempty" yaml:"parameter_id,omitempty,flow"`
	Data        string `json:"data,omitempty" yaml:"data,omitempty,flow"`
}

func decodeComStmtSendLongData(packet []byte) (COM_STMT_SEND_LONG_DATA, error) {
	if len(packet) < 7 || packet[0] != 0x18 {
		return COM_STMT_SEND_LONG_DATA{}, fmt.Errorf("invalid COM_STMT_SEND_LONG_DATA packet")
	}
	stmtID := binary.LittleEndian.Uint32(packet[1:5])
	paramID := binary.LittleEndian.Uint16(packet[5:7])
	data := packet[7:]
	return COM_STMT_SEND_LONG_DATA{
		StatementID: stmtID,
		ParameterID: paramID,
		Data:        base64.StdEncoding.EncodeToString(data),
	}, nil
}
