package models

import (
	"gopkg.in/yaml.v3"
)

type MySQLSpec struct {
	Metadata  map[string]string   `json:"metadata" yaml:"metadata"`
	Requests  []MysqlRequestYaml  `json:"requests" yaml:"requests"`
	Response  []MysqlResponseYaml `json:"responses" yaml:"responses"`
	CreatedAt int64               `json:"created" yaml:"created,omitempty"`
}

type MysqlRequestYaml struct {
	Header    *MySQLPacketHeader `json:"header,omitempty" yaml:"header"`
	Message   yaml.Node          `json:"message,omitempty" yaml:"message"`
	ReadDelay int64              `json:"read_delay,omitempty" yaml:"read_delay,omitempty"`
}

type MysqlResponseYaml struct {
	Header    *MySQLPacketHeader `json:"header,omitempty" yaml:"header"`
	Message   yaml.Node          `json:"message,omitempty" yaml:"message"`
	ReadDelay int64              `json:"read_delay,omitempty" yaml:"read_delay,omitempty"`
}

type MySQLPacketHeader struct {
	PacketLength uint32 `json:"packet_length,omitempty" yaml:"packet_length,omitempty,flow" bson:"packet_length,omitempty"`
	PacketNumber uint8  `json:"packet_number,omitempty" yaml:"packet_number,omitempty,flow" bson:"packet_number,omitempty"`
	PacketType   string `json:"packet_type,omitempty" yaml:"packet_type,omitempty,flow" bson:"packet_type,omitempty"`
}

type MySQLRequest struct {
	Header    *MySQLPacketHeader `json:"header,omitempty" yaml:"header,omitempty,flow" bson:"header,omitempty"`
	Message   interface{}        `json:"message,omitempty" yaml:"message,omitempty,flow" bson:"message,omitempty"`
	ReadDelay int64              `json:"read_delay,omitempty" yaml:"read_delay,omitempty,flow" bson:"read_delay,omitempty"`
}

// func (mr *MySQLRequest) UnmarshalBSON(data []byte) error
// func (mr *MySQLRequest) UnmarshalJSON(data []byte) error
// func (mr *MySQLRequest) MarshalJSON() ([]byte, error)

type RowColumnDefinition struct {
	Type  FieldType   `json:"type,omitempty" yaml:"type,omitempty,flow" bson:"type,omitempty"`
	Name  string      `json:"name,omitempty" yaml:"name,omitempty,flow" bson:"name,omitempty"`
	Value interface{} `json:"value,omitempty" yaml:"value,omitempty,flow" bson:"value,omitempty"`
}

// func (r *RowColumnDefinition) UnmarshalBSON(data []byte) error
// func (mr *RowColumnDefinition) UnmarshalJSON(data []byte) error
// func (mr *RowColumnDefinition) MarshalJSON() ([]byte, error)

type MySQLResponse struct {
	Header    *MySQLPacketHeader `json:"header,omitempty" yaml:"header,omitempty,flow" bson:"header,omitempty"`
	Message   interface{}        `json:"message,omitempty" yaml:"message,omitempty,flow" bson:"message,omitempty"`
	ReadDelay int64              `json:"read_delay,omitempty" yaml:"read_delay,omitempty,flow" bson:"read_delay,omitempty"`
}

// func (mr *MySQLResponse) UnmarshalBSON(data []byte) error
// func (mr *MySQLResponse) UnmarshalJSON(data []byte) error
// func (mr *MySQLResponse) MarshalJSON() ([]byte, error)

type MySQLHandshakeV10Packet struct {
	ProtocolVersion uint8  `json:"protocol_version,omitempty" yaml:"protocol_version,omitempty,flow" bson:"protocol_version,omitempty"`
	ServerVersion   string `json:"server_version,omitempty" yaml:"server_version,omitempty,flow" bson:"server_version,omitempty"`
	ConnectionID    uint32 `json:"connection_id,omitempty" yaml:"connection_id,omitempty,flow" bson:"connection_id,omitempty"`
	AuthPluginData  string `json:"auth_plugin_data,omitempty" yaml:"auth_plugin_data,omitempty,flow" bson:"auth_plugin_data,omitempty"`
	CapabilityFlags uint32 `json:"capability_flags,omitempty" yaml:"capability_flags,omitempty,flow" bson:"capability_flags,omitempty"`
	CharacterSet    uint8  `json:"character_set,omitempty" yaml:"character_set,omitempty,flow" bson:"character_set,omitempty"`
	StatusFlags     uint16 `json:"status_flags,omitempty" yaml:"status_flags,omitempty,flow" bson:"status_flags,omitempty"`
	AuthPluginName  string `json:"auth_plugin_name,omitempty" yaml:"auth_plugin_name,omitempty,flow" bson:"auth_plugin_name,omitempty"`
}

type PluginDetails struct {
	Type    string `json:"type,omitempty" yaml:"type,omitempty,flow" bson:"type,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty,flow" bson:"message,omitempty"`
}

type MySQLHandshakeResponseOk struct {
	PacketIndicator string        `json:"packet_indicator,omitempty" yaml:"packet_indicator,omitempty,flow" bson:"packet_indicator,omitempty"`
	PluginDetails   PluginDetails `json:"plugin_details,omitempty" yaml:"plugin_details,omitempty,flow" bson:"plugin_details,omitempty"`
	RemainingBytes  string        `json:"remaining_bytes,omitempty" yaml:"remaining_bytes,omitempty,flow" bson:"remaining_bytes,omitempty"`
}

type MySQLHandshakeResponse struct {
	CapabilityFlags uint32 `json:"capability_flags,omitempty" yaml:"capability_flags,omitempty,flow" bson:"capability_flags,omitempty"`
	MaxPacketSize   uint32 `json:"max_packet_size,omitempty" yaml:"max_packet_size,omitempty,flow" bson:"max_packet_size,omitempty"`
	CharacterSet    uint8  `json:"character_set,omitempty" yaml:"character_set,omitempty,flow" bson:"character_set,omitempty"`
	Reserved        int    `json:"reserved,omitempty" yaml:"reserved,omitempty,flow" bson:"reserved,omitempty"`
	Username        string `json:"username,omitempty" yaml:"username,omitempty,flow" bson:"username,omitempty"`
	AuthData        string `json:"auth_data,omitempty" yaml:"auth_data,omitempty,flow" bson:"auth_data,omitempty"`
	Database        string `json:"database,omitempty" yaml:"database,omitempty,flow" bson:"database,omitempty"`
	AuthPluginName  string `json:"auth_plugin_name,omitempty" yaml:"auth_plugin_name,omitempty,flow" bson:"auth_plugin_name,omitempty"`
}

type MySQLQueryPacket struct {
	Command byte   `json:"command,omitempty" yaml:"command,omitempty,flow" bson:"command,omitempty"`
	Query   string `json:"query,omitempty" yaml:"query,omitempty,flow" bson:"query,omitempty"`
}

type MySQLComStmtExecute struct {
	StatementID    uint32           `json:"statement_id,omitempty" yaml:"statement_id,omitempty,flow" bson:"statement_id,omitempty"`
	Flags          byte             `json:"flags,omitempty" yaml:"flags,omitempty,flow" bson:"flags,omitempty"`
	IterationCount uint32           `json:"iteration_count,omitempty" yaml:"iteration_count,omitempty,flow" bson:"iteration_count,omitempty"`
	NullBitmap     string           `json:"null_bitmap,omitempty" yaml:"null_bitmap,omitempty,flow" bson:"null_bitmap,omitempty"`
	ParamCount     uint16           `json:"param_count,omitempty" yaml:"param_count,omitempty,flow" bson:"param_count,omitempty"`
	Parameters     []BoundParameter `json:"parameters,omitempty" yaml:"parameters,omitempty,flow" bson:"parameters,omitempty"`
}

type BoundParameter struct {
	Type     byte   `json:"type,omitempty" yaml:"type,omitempty,flow" bson:"type,omitempty"`
	Unsigned byte   `json:"unsigned,omitempty" yaml:"unsigned,omitempty,flow" bson:"unsigned,omitempty"`
	Value    []byte `json:"value,omitempty" yaml:"value,omitempty,flow" bson:"value,omitempty"`
}

type MySQLStmtPrepareOk struct {
	Status       byte               `json:"status,omitempty" yaml:"status,omitempty,flow" bson:"status,omitempty"`
	StatementID  uint32             `json:"statement_id,omitempty" yaml:"statement_id,omitempty,flow" bson:"statement_id,omitempty"`
	NumColumns   uint16             `json:"num_columns,omitempty" yaml:"num_columns,omitempty,flow" bson:"num_columns,omitempty"`
	NumParams    uint16             `json:"num_params,omitempty" yaml:"num_params,omitempty,flow" bson:"num_params,omitempty"`
	WarningCount uint16             `json:"warning_count,omitempty" yaml:"warning_count,omitempty,flow" bson:"warning_count,omitempty"`
	ColumnDefs   []ColumnDefinition `json:"column_definitions,omitempty" yaml:"column_definitions,omitempty,flow" bson:"column_definitions,omitempty"`
	ParamDefs    []ColumnDefinition `json:"param_definitions,omitempty" yaml:"param_definitions,omitempty,flow" bson:"param_definitions,omitempty"`
}
type MySQLResultSet struct {
	Columns             []*ColumnDefinition `json:"columns,omitempty" yaml:"columns,omitempty,flow" bson:"columns,omitempty"`
	Rows                []*Row              `json:"rows,omitempty" yaml:"rows,omitempty,flow" bson:"rows,omitempty"`
	EOFPresent          bool                `json:"eofPresent,omitempty" yaml:"eofPresent,omitempty,flow" bson:"eofPresent,omitempty"`
	PaddingPresent      bool                `json:"paddingPresent,omitempty" yaml:"paddingPresent,omitempty,flow" bson:"paddingPresent,omitempty"`
	EOFPresentFinal     bool                `json:"eofPresentFinal,omitempty" yaml:"eofPresentFinal,omitempty,flow" bson:"eofPresentFinal,omitempty"`
	PaddingPresentFinal bool                `json:"paddingPresentFinal,omitempty" yaml:"paddingPresentFinal,omitempty,flow" bson:"paddingPresentFinal,omitempty"`
	OptionalPadding     bool                `json:"optionalPadding,omitempty" yaml:"optionalPadding,omitempty,flow" bson:"optionalPadding,omitempty"`
	OptionalEOFBytes    string              `json:"optionalEOFBytes,omitempty" yaml:"optionalEOFBytes,omitempty,flow" bson:"optionalEOFBytes,omitempty"`
	EOFAfterColumns     string              `json:"eofAfterColumns,omitempty" yaml:"eofAfterColumns,omitempty,flow" bson:"eofAfterColumns,omitempty"`
}

type PacketHeader struct {
	PacketLength     uint32 `json:"packet_length,omitempty" yaml:"packet_length,omitempty,flow" bson:"packet_length,omitempty"`
	PacketSequenceID uint8  `json:"packet_sequence_id,omitempty" yaml:"packet_sequence_id,omitempty,flow" bson:"packet_sequence_id,omitempty"`
}

type RowHeader struct {
	PacketLength     uint8 `json:"packet_length,omitempty" yaml:"packet_length,omitempty,flow" bson:"packet_length,omitempty"`
	PacketSequenceID uint8 `json:"packet_sequence_id,omitempty" yaml:"packet_sequence_id,omitempty,flow" bson:"packet_sequence_id,omitempty"`
}

type ColumnDefinition struct {
	Catalog      string       `json:"catalog,omitempty" yaml:"catalog,omitempty,flow" bson:"catalog,omitempty"`
	Schema       string       `json:"schema,omitempty" yaml:"schema,omitempty,flow" bson:"schema,omitempty"`
	Table        string       `json:"table,omitempty" yaml:"table,omitempty,flow" bson:"table,omitempty"`
	OrgTable     string       `json:"org_table,omitempty" yaml:"org_table,omitempty,flow" bson:"org_table,omitempty"`
	Name         string       `json:"name,omitempty" yaml:"name,omitempty,flow" bson:"name,omitempty"`
	OrgName      string       `json:"org_name,omitempty" yaml:"org_name,omitempty,flow" bson:"org_name,omitempty"`
	NextLength   uint64       `json:"next_length,omitempty" yaml:"next_length,omitempty,flow" bson:"next_length,omitempty"`
	CharacterSet uint16       `json:"character_set,omitempty" yaml:"character_set,omitempty,flow" bson:"character_set,omitempty"`
	ColumnLength uint32       `json:"column_length,omitempty" yaml:"column_length,omitempty,flow" bson:"column_length,omitempty"`
	ColumnType   byte         `json:"column_type,omitempty" yaml:"column_type,omitempty,flow" bson:"column_type,omitempty"`
	Flags        uint16       `json:"flags,omitempty" yaml:"flags,omitempty,flow" bson:"flags,omitempty"`
	Decimals     byte         `json:"decimals,omitempty" yaml:"decimals,omitempty,flow" bson:"decimals,omitempty"`
	PacketHeader PacketHeader `json:"packet_header,omitempty" yaml:"packet_header,omitempty,flow" bson:"packet_header,omitempty"`
}

type Row struct {
	Header  RowHeader             `json:"header,omitempty" yaml:"header,omitempty,flow" bson:"header,omitempty"`
	Columns []RowColumnDefinition `json:"columns,omitempty" yaml:"columns,omitempty,flow" bson:"columns,omitempty"`
}

type MySQLOKPacket struct {
	AffectedRows uint64 `json:"affected_rows,omitempty" yaml:"affected_rows,omitempty,flow" bson:"affected_rows,omitempty"`
	LastInsertID uint64 `json:"last_insert_id,omitempty" yaml:"last_insert_id,omitempty,flow" bson:"last_insert_id,omitempty"`
	StatusFlags  uint16 `json:"status_flags,omitempty" yaml:"status_flags,omitempty,flow" bson:"status_flags,omitempty"`
	Warnings     uint16 `json:"warnings,omitempty" yaml:"warnings,omitempty,flow" bson:"warnings,omitempty"`
	Info         string `json:"info,omitempty" yaml:"info,omitempty,flow" bson:"info,omitempty"`
}

type MySQLERRPacket struct {
	Header         byte   `json:"header,omitempty" yaml:"header,omitempty,flow" bson:"header,omitempty"`
	ErrorCode      uint16 `json:"error_code,omitempty" yaml:"error_code,omitempty,flow" bson:"error_code,omitempty"`
	SQLStateMarker string `json:"sql_state_marker,omitempty" yaml:"sql_state_marker,omitempty,flow" bson:"sql_state_marker,omitempty"`
	SQLState       string `json:"sql_state,omitempty" yaml:"sql_state,omitempty,flow" bson:"sql_state,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty" yaml:"error_message,omitempty,flow" bson:"error_message,omitempty"`
}

type MySQLComStmtPreparePacket struct {
	Query string `json:"query,omitempty" yaml:"query,omitempty,flow" bson:"query,omitempty"`
}

type MySQLComStmtSendLongData struct {
	StatementID uint32 `json:"statement_id,omitempty" yaml:"statement_id,omitempty,flow" bson:"statement_id,omitempty"`
	ParameterID uint16 `json:"parameter_id,omitempty" yaml:"parameter_id,omitempty,flow" bson:"parameter_id,omitempty"`
	Data        string `json:"data,omitempty" yaml:"data,omitempty,flow" bson:"data,omitempty"`
}

type MySQLcomStmtReset struct {
	StatementID uint32 `json:"statement_id,omitempty" yaml:"statement_id,omitempty,flow" bson:"statement_id,omitempty"`
}

type MySQLComStmtFetchPacket struct {
	StatementID uint32 `json:"statement_id,omitempty" yaml:"statement_id,omitempty,flow" bson:"statement_id,omitempty"`
	RowCount    uint32 `json:"row_count,omitempty" yaml:"row_count,omitempty,flow" bson:"row_count,omitempty"`
	Info        string `json:"info,omitempty" yaml:"info,omitempty,flow" bson:"info,omitempty"`
}

type MySQLComChangeUserPacket struct {
	User         string `json:"user,omitempty" yaml:"user,omitempty,flow" bson:"user,omitempty"`
	Auth         string `json:"auth,omitempty" yaml:"auth,omitempty,flow" bson:"auth,omitempty"`
	Db           string `json:"db,omitempty" yaml:"db,omitempty,flow" bson:"db,omitempty"`
	CharacterSet uint8  `json:"character_set,omitempty" yaml:"character_set,omitempty,flow" bson:"character_set,omitempty"`
	AuthPlugin   string `json:"auth_plugin,omitempty" yaml:"auth_plugin,omitempty,flow" bson:"auth_plugin,omitempty"`
}

type MySQLComStmtClosePacket struct {
	StatementID uint32 `json:"statement_id,omitempty" yaml:"statement_id,omitempty,flow" bson:"statement_id,omitempty"`
}

type AuthSwitchResponsePacket struct {
	AuthResponseData string `json:"auth_response_data,omitempty" yaml:"auth_response_data,omitempty,flow" bson:"auth_response_data,omitempty"`
}

type AuthSwitchRequestPacket struct {
	StatusTag      byte   `json:"status_tag,omitempty" yaml:"status_tag,omitempty,flow" bson:"status_tag,omitempty"`
	PluginName     string `json:"plugin_name,omitempty" yaml:"plugin_name,omitempty,flow" bson:"plugin_name,omitempty"`
	PluginAuthData string `json:"plugin_authdata,omitempty" yaml:"plugin_authdata,omitempty,flow" bson:"plugin_authdata,omitempty"`
}
