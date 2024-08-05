//go:build linux

package decoder

import (
	"context"
	"fmt"
	"io"
	"net"

	"go.keploy.io/server/v2/pkg/core/proxy/integrations"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations/mysql/operation"
	mysqlUtils "go.keploy.io/server/v2/pkg/core/proxy/integrations/mysql/utils"
	pUtil "go.keploy.io/server/v2/pkg/core/proxy/util"
	"go.keploy.io/server/v2/pkg/models"
	"go.keploy.io/server/v2/pkg/models/mysql"
	"go.keploy.io/server/v2/utils"
	"go.uber.org/zap"
)

func simulateCommandPhase(ctx context.Context, logger *zap.Logger, clientConn net.Conn, mockDb integrations.MockMemDb, decodeCtx *operation.DecodeContext, dstCfg *integrations.ConditionalDstCfg, opts models.OutgoingOptions) error {

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:

			// read the command from the client
			command, err := mysqlUtils.ReadPacketBuffer(ctx, logger, clientConn)
			if err != nil {
				if err != io.EOF {
					utils.LogError(logger, err, "failed to read command packet from client")
				}
				return err
			}

			// Decode the command
			commandPkt, err := operation.DecodePayload(ctx, logger, command, clientConn, decodeCtx)
			if err != nil {
				utils.LogError(logger, err, "failed to decode the MySQL packet from the client")
			}

			req := mysql.Request{
				PacketBundle: *commandPkt,
			}

			// Match the request with the mock
			resp, ok, err := matchCommand(ctx, logger, req, mockDb, decodeCtx)
			if err != nil {
				utils.LogError(logger, err, "failed to match the command")
				return err
			}

			if !ok {
				utils.LogError(logger, nil, "No matching mock found for the command", zap.Any("command", command))
				if opts.FallBackOnMiss {
					_, err = pUtil.PassThrough(ctx, logger, clientConn, dstCfg, [][]byte{command})
					if err != nil {
						utils.LogError(logger, err, "failed to passThrough mysql request", zap.Any("command", command))
						return err
					}
				}
				return fmt.Errorf("no matching mock found for the command")
			}

			logger.Debug("Matched the command with the mock", zap.Any("mock", resp))

			//Encode the matched resp
			buf, err := operation.EncodeToBinary(ctx, logger, &resp.PacketBundle, clientConn, decodeCtx)
			if err != nil {
				utils.LogError(logger, err, "failed to encode the response", zap.Any("response", resp))
			}

			if operation.IsNoResponseCommand(commandPkt.Header.Type) {
				// No response for COM_STMT_CLOSE and COM_STMT_SEND_LONG_DATA
				logger.Debug("No response for the command", zap.Any("command", command))
				continue
			}

			// Write the response to the client
			_, err = clientConn.Write(buf)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				utils.LogError(logger, err, "failed to write the response to the client")
				return err
			}
		}
	}
}
