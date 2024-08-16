//go:build linux

package recorder

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	mysqlUtils "go.keploy.io/server/v2/pkg/core/proxy/integrations/mysql/utils"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations/mysql/wire"
	intgUtils "go.keploy.io/server/v2/pkg/core/proxy/integrations/util"
	pTls "go.keploy.io/server/v2/pkg/core/proxy/tls"
	"go.keploy.io/server/v2/pkg/models"
	"go.keploy.io/server/v2/pkg/models/mysql"
	"go.keploy.io/server/v2/utils"
	"go.uber.org/zap"
)

// Record mode
type handshakeRes struct {
	req               []mysql.Request
	resp              []mysql.Response
	requestOperation  string
	responseOperation string
	reqTimestamp      time.Time
	tlsClientConn     net.Conn
	tlsDestConn       net.Conn
}

// Experiment:
type Conn struct {
	net.Conn
	r      io.Reader
	logger *zap.Logger
	mu     sync.Mutex
}

func (c *Conn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(p) == 0 {
		c.logger.Debug("the length is 0 for the reading from customConn")
	}
	return c.r.Read(p)
}

///

func handleInitialHandshake(ctx context.Context, logger *zap.Logger, clientConn, destConn net.Conn, decodeCtx *wire.DecodeContext, opts models.OutgoingOptions) (handshakeRes, error) {

	res := handshakeRes{
		req:  make([]mysql.Request, 0),
		resp: make([]mysql.Response, 0),
	}

	// Read the initial handshake from the server (server-greetings)
	handshake, err := mysqlUtils.ReadPacketBuffer(ctx, logger, destConn)
	if err != nil {
		utils.LogError(logger, err, "failed to read initial handshake from server")
		return res, err
	}

	// Write the initial handshake to the client
	_, err = clientConn.Write(handshake)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write server greetings to the client")

		return res, err
	}

	// Set the timestamp of the initial request
	res.reqTimestamp = time.Now()

	// Decode server handshake packet
	handshakePkt, err := wire.DecodePayload(ctx, logger, handshake, clientConn, decodeCtx)
	if err != nil {
		utils.LogError(logger, err, "failed to decode handshake packet")
		return res, err
	}

	// Set the intial request operation
	res.requestOperation = handshakePkt.Header.Type

	// Get the initial Plugin Name
	pluginName, err := wire.GetPluginName(handshakePkt.Message)
	if err != nil {
		utils.LogError(logger, err, "failed to get initial plugin name")
		return res, err
	}

	// Set the initial plugin name
	decodeCtx.PluginName = pluginName

	res.resp = append(res.resp, mysql.Response{
		PacketBundle: *handshakePkt,
	})

	// Handshake response from client (or SSL request)
	handshakeResponse, err := mysqlUtils.ReadPacketBuffer(ctx, logger, clientConn)
	if err != nil {
		if err == io.EOF {
			logger.Debug("received request buffer is empty in record mode for mysql call")
			return res, err
		}
		utils.LogError(logger, err, "failed to read handshake response from client")

		return res, err
	}

	_, err = destConn.Write(handshakeResponse)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write handshake response to server")

		return res, err
	}

	// Decode client handshake response packet
	handshakeResponsePkt, err := wire.DecodePayload(ctx, logger, handshakeResponse, clientConn, decodeCtx)
	if err != nil {
		utils.LogError(logger, err, "failed to decode handshake response packet")
		return res, err
	}

	res.req = append(res.req, mysql.Request{
		PacketBundle: *handshakeResponsePkt,
	})

	if decodeCtx.UseSSL {
		reader := bufio.NewReader(clientConn)
		initialData := make([]byte, 5)
		// reading the initial data from the client connection to determine if the connection is a TLS handshake
		testBuffer, err := reader.Peek(len(initialData))
		if err != nil {
			if err == io.EOF && len(testBuffer) == 0 {
				logger.Debug("received EOF, closing conn", zap.Error(err))
				return res, nil
			}
			utils.LogError(logger, err, "failed to peek the request message in proxy")
			return res, err
		}

		multiReader := io.MultiReader(reader, clientConn)
		clientConn = &Conn{
			Conn:   clientConn,
			r:      multiReader,
			logger: logger,
		}

		isTLS := pTls.IsTLSHandshake(testBuffer)
		if isTLS {
			clientConn, err = pTls.HandleTLSConnection(ctx, logger, clientConn)
			if err != nil {
				utils.LogError(logger, err, "failed to handle TLS conn")
				return res, err
			}
		}

		// upgrade the destConn to TLS if the client connection is upgraded to TLS
		var tlsDestConn *tls.Conn
		if isTLS {
			fmt.Printf("Upgrading the destination connection to TLS\n")
			fmt.Printf("Tls addr and config: %v, %v\n", opts.DstCfg.Addr, opts.DstCfg.TLSCfg)
			fmt.Printf("Certificate: %v\n", opts.DstCfg.TLSCfg)
			tlsDestConn = tls.Client(destConn, opts.DstCfg.TLSCfg)
			err = tlsDestConn.Handshake()
			if err != nil {
				utils.LogError(logger, err, "failed to upgrade the destination connection to TLS")
				return res, err
			}
			logger.Debug("TLS connection established with the destination server", zap.Any("Destination Addr", destConn.RemoteAddr().String()))
		}

		// Update the destination connection to TLS connection
		destConn = tlsDestConn

		// Update this tls connection information in the handshake result
		res.tlsClientConn = clientConn
		res.tlsDestConn = destConn

		// Store the last operation for the upgraded client connection
		decodeCtx.LastOp.Store(clientConn, mysql.HandshakeV10)
		// Store the server greeting packet for the upgraded client connection

		sg, ok := handshakePkt.Message.(*mysql.HandshakeV10Packet)
		if !ok {
			return res, fmt.Errorf("failed to type assert handshake packet")
		}
		decodeCtx.ServerGreetings.Store(clientConn, sg)

		handshakeResponse, err := mysqlUtils.ReadPacketBuffer(ctx, logger, clientConn)
		if err != nil {
			if err == io.EOF {
				logger.Debug("received request buffer is empty in record mode for mysql call")
				return res, err
			}
			utils.LogError(logger, err, "failed to read handshake response from client")

			return res, err
		}

		println("Before writing handshake response to server")
		_, err = destConn.Write(handshakeResponse)
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			utils.LogError(logger, err, "failed to write handshake response to server")

			return res, err
		}
		println("After writing handshake response to server")

		// Decode client handshake response packet
		handshakeResponsePkt, err := wire.DecodePayload(ctx, logger, handshakeResponse, clientConn, decodeCtx)
		if err != nil {
			utils.LogError(logger, err, "failed to decode handshake response packet")
			return res, err
		}

		res.req = append(res.req, mysql.Request{
			PacketBundle: *handshakeResponsePkt,
		})
	}

	// Read the next auth packet, It can be either auth more data or auth switch request in case of caching_sha2_password
	// or it can be OK packet in case of native password
	authData, err := mysqlUtils.ReadPacketBuffer(ctx, logger, destConn)
	if err != nil {
		if err == io.EOF {
			logger.Debug("received request buffer is empty in record mode for mysql call")

			return res, err
		}
		utils.LogError(logger, err, "failed to read auth or final response packet from server during handshake")
		return res, err
	}

	// AuthSwitchRequest: If the server sends an AuthSwitchRequest, then there must be a diff auth type with its data
	// AuthMoreData: If the server sends an AuthMoreData, then it tells the auth mechanism type for the initial plugin name
	// OK/ERR: If the server sends an OK/ERR packet, in case of native password.
	_, err = clientConn.Write(authData)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write auth packet to client during handshake")
		return res, err
	}

	// Decode auth or final response packet
	authDecider, err := wire.DecodePayload(ctx, logger, authData, clientConn, decodeCtx)
	if err != nil {
		utils.LogError(logger, err, "failed to decode auth more data packet")
		return res, err
	}

	var authRes handshakeRes
	switch authDecider.Message.(type) {
	case *mysql.AuthSwitchRequestPacket:
		pkt := authDecider.Message.(*mysql.AuthSwitchRequestPacket)

		// Change the plugin name due to auth switch request
		decodeCtx.PluginName = pkt.PluginName

		authRes, err = handleAuth(ctx, logger, authDecider, clientConn, destConn, decodeCtx)
		if err != nil {
			return res, fmt.Errorf("failed to handle auth switch request: %w", err)
		}
	case *mysql.AuthMoreDataPacket:
		authRes, err = handleAuth(ctx, logger, authDecider, clientConn, destConn, decodeCtx)
		if err != nil {
			return res, fmt.Errorf("failed to handle auth more data: %w", err)
		}
	case *mysql.OKPacket:
		authRes, err = handleAuth(ctx, logger, authDecider, clientConn, destConn, decodeCtx)
		if err != nil {
			return res, fmt.Errorf("failed to handle ok packet: %w", err)
		}
	}

	setHandshakeResult(&res, authRes)

	return res, nil
}

func setHandshakeResult(res *handshakeRes, authRes handshakeRes) {
	res.req = append(res.req, authRes.req...)
	res.resp = append(res.resp, authRes.resp...)
	res.responseOperation = authRes.responseOperation
}

func handleAuth(ctx context.Context, logger *zap.Logger, authPkt *mysql.PacketBundle, clientConn, destConn net.Conn, decodeCtx *wire.DecodeContext) (handshakeRes, error) {
	res := handshakeRes{
		req:  make([]mysql.Request, 0),
		resp: make([]mysql.Response, 0),
	}

	switch mysql.AuthPluginName(decodeCtx.PluginName) {
	case mysql.Native:
		res.resp = append(res.resp, mysql.Response{
			PacketBundle: *authPkt,
		})

		res.responseOperation = authPkt.Header.Type
		logger.Debug("native password authentication is handled successfully")
	case mysql.CachingSha2:
		result, err := handleCachingSha2Password(ctx, logger, authPkt, clientConn, destConn, decodeCtx)
		if err != nil {
			return res, fmt.Errorf("failed to handle caching sha2 password: %w", err)
		}
		setHandshakeResult(&res, result)
	case mysql.Sha256:
		return res, fmt.Errorf("Sha256 Password authentication is not supported")
	default:
		return res, fmt.Errorf("unsupported authentication plugin: %s", decodeCtx.PluginName)
	}

	return res, nil
}

func handleCachingSha2Password(ctx context.Context, logger *zap.Logger, authPkt *mysql.PacketBundle, clientConn, destConn net.Conn, decodeCtx *wire.DecodeContext) (handshakeRes, error) {
	res := handshakeRes{
		req:  make([]mysql.Request, 0),
		resp: make([]mysql.Response, 0),
	}

	var authMechanism string
	var err error
	switch authPkt.Message.(type) {
	case *mysql.AuthSwitchRequestPacket:
		pkt := authPkt.Message.(*mysql.AuthSwitchRequestPacket)
		//Change the plugin data to a string for better readability as it is expected to be either "full auth" or "fast auth success"
		authMechanism, err = wire.GetCachingSha2PasswordMechanism(pkt.PluginData[0])
		if err != nil {
			return res, fmt.Errorf("failed to get caching sha2 password mechanism: %w", err)
		}
		pkt.PluginData = authMechanism

	case *mysql.AuthMoreDataPacket:
		authMorePkt := authPkt.Message.(*mysql.AuthMoreDataPacket)
		// Getting the string value of the caching_sha2_password mechanism
		authMechanism, err = wire.GetCachingSha2PasswordMechanism(authMorePkt.Data[0])
		if err != nil {
			return res, fmt.Errorf("failed to get caching sha2 password mechanism: %w", err)
		}
		authMorePkt.Data = authMechanism
	}

	// save the auth more data or auth switch request packet
	res.resp = append(res.resp, mysql.Response{
		PacketBundle: *authPkt,
	})

	auth, err := wire.StringToCachingSha2PasswordMechanism(authMechanism)
	if err != nil {
		return res, fmt.Errorf("failed to convert string to caching sha2 password mechanism: %w", err)
	}

	var result handshakeRes
	switch auth {
	case mysql.PerformFullAuthentication:
		result, err = handleFullAuth(ctx, logger, clientConn, destConn, decodeCtx)
		if err != nil {
			return res, fmt.Errorf("failed to handle caching sha2 password full auth: %w", err)
		}
	case mysql.FastAuthSuccess:
		result, err = handleFastAuthSuccess(ctx, logger, clientConn, destConn, decodeCtx)
		if err != nil {
			return res, fmt.Errorf("failed to handle caching sha2 password fast auth success: %w", err)
		}
	}

	setHandshakeResult(&res, result)

	return res, nil
}

func handleFastAuthSuccess(ctx context.Context, logger *zap.Logger, clientConn, destConn net.Conn, decodeCtx *wire.DecodeContext) (handshakeRes, error) {
	res := handshakeRes{
		req:  make([]mysql.Request, 0),
		resp: make([]mysql.Response, 0),
	}

	//As per wire shark capture, during fast auth success, server sends OK packet just after auth more data

	// read the ok/err packet from the server after auth more data
	finalResp, err := mysqlUtils.ReadPacketBuffer(ctx, logger, destConn)
	if err != nil {
		if err == io.EOF {
			logger.Debug("received request buffer is empty in record mode for mysql call")
			return res, err
		}
		utils.LogError(logger, err, "failed to read final response packet from server")
		return res, err
	}

	// write the ok/err packet to the client
	_, err = clientConn.Write(finalResp)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write ok/err packet to client during fast auth mechanism")
		return res, err
	}

	finalPkt, err := wire.DecodePayload(ctx, logger, finalResp, clientConn, decodeCtx)
	if err != nil {
		utils.LogError(logger, err, "failed to decode final response packet after auth data packet")
		return res, err
	}

	res.resp = append(res.resp, mysql.Response{
		PacketBundle: *finalPkt,
	})

	// Set the final response operation of the handshake
	res.responseOperation = finalPkt.Header.Type
	logger.Debug("fast auth success is handled successfully")

	return res, nil
}

func handleFullAuth(ctx context.Context, logger *zap.Logger, clientConn, destConn net.Conn, decodeCtx *wire.DecodeContext) (handshakeRes, error) {
	res := handshakeRes{
		req:  make([]mysql.Request, 0),
		resp: make([]mysql.Response, 0),
	}

	// read the public key request from the client
	publicKeyRequest, err := mysqlUtils.ReadPacketBuffer(ctx, logger, clientConn)
	if err != nil {
		utils.LogError(logger, err, "failed to read public key request from client")
		return res, err
	}
	_, err = destConn.Write(publicKeyRequest)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write public key request to server")
		return res, err
	}

	publicKeyReqPkt, err := wire.DecodePayload(ctx, logger, publicKeyRequest, clientConn, decodeCtx)
	if err != nil {
		utils.LogError(logger, err, "failed to decode public key request packet")
		return res, err
	}

	res.req = append(res.req, mysql.Request{
		PacketBundle: *publicKeyReqPkt,
	})

	// read the "public key" as response from the server
	pubKey, err := mysqlUtils.ReadPacketBuffer(ctx, logger, destConn)
	if err != nil {
		utils.LogError(logger, err, "failed to read public key from server")
		return res, err
	}
	_, err = clientConn.Write(pubKey)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write public key response to client")
		return res, err
	}

	pubKeyPkt, err := wire.DecodePayload(ctx, logger, pubKey, clientConn, decodeCtx)
	if err != nil {
		utils.LogError(logger, err, "failed to decode public key packet")
		return res, err
	}

	pubKeyPkt.Meta = map[string]string{
		"auth operation": "public key response",
	}

	res.resp = append(res.resp, mysql.Response{
		PacketBundle: *pubKeyPkt,
	})

	// read the encrypted password from the client
	encryptPass, err := mysqlUtils.ReadPacketBuffer(ctx, logger, clientConn)
	if err != nil {
		utils.LogError(logger, err, "failed to read encrypted password from client")

		return res, err
	}
	_, err = destConn.Write(encryptPass)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write encrypted password to server")
		return res, err
	}

	encPass, err := mysqlUtils.BytesToMySQLPacket(encryptPass)
	if err != nil {
		utils.LogError(logger, err, "failed to parse MySQL packet")
		return res, err
	}

	encryptPassPkt := &mysql.PacketBundle{
		Header: &mysql.PacketInfo{
			Header: &encPass.Header,
			Type:   mysql.EncryptedPassword,
		},
		Message: intgUtils.EncodeBase64(encPass.Payload),
	}

	res.req = append(res.req, mysql.Request{
		PacketBundle: *encryptPassPkt,
	})

	// read the final response from the server (ok or error)
	finalServerResponse, err := mysqlUtils.ReadPacketBuffer(ctx, logger, destConn)
	if err != nil {
		utils.LogError(logger, err, "failed to read final response from server")
		return res, err
	}
	_, err = clientConn.Write(finalServerResponse)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		utils.LogError(logger, err, "failed to write final response to client")

		return res, err
	}

	finalResPkt, err := wire.DecodePayload(ctx, logger, finalServerResponse, clientConn, decodeCtx)

	if err != nil {
		utils.LogError(logger, err, "failed to decode final response packet during caching sha2 password full auth")
		return res, err
	}

	res.resp = append(res.resp, mysql.Response{
		PacketBundle: *finalResPkt,
	})

	// Set the final response operation of the handshake
	res.responseOperation = finalResPkt.Header.Type

	logger.Debug("full auth is handled successfully")
	return res, nil
}
