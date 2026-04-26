package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/snestors/agenthub/internal/ws"
)

func (s *Server) registerWSActions() {
	s.hub.HandleAction("send_message", s.handleWSSendMessage)
	s.hub.HandleAction("set_engine", s.handleWSSetEngine)
	s.hub.HandleAction("service_action", s.handleWSServiceAction)
}

func (s *Server) handleWSSendMessage(ctx context.Context, c *ws.Client, act ws.ClientAction) (*ws.Envelope, error) {
	req, err := sendMsgReqFromAction(act)
	if err == nil {
		var res sendMessageAccepted
		res, err = s.acceptMessage(ctx, req)
		if err == nil {
			ack := wsAck("agent", act.ID, res, nil)
			c.SendDirect(ack)
			return nil, nil
		}
	}
	ack := wsAck("agent", act.ID, nil, err)
	c.SendDirect(ack)
	return nil, nil
}

func (s *Server) handleWSSetEngine(ctx context.Context, c *ws.Client, act ws.ClientAction) (*ws.Envelope, error) {
	res, err := s.setEngine(ctx, engineSetReq{Engine: act.Engine, Model: act.Model})
	c.SendDirect(wsAck("agent", act.ID, res, err))
	if err == nil {
		go s.broadcastAgentStatus(context.Background())
	}
	return nil, nil
}

func (s *Server) handleWSServiceAction(ctx context.Context, c *ws.Client, act ws.ClientAction) (*ws.Envelope, error) {
	res, err := s.serviceAction(ctx, act.Name, act.Op)
	c.SendDirect(wsAck("system", act.ID, res, err))
	result := "ok"
	errMsg := ""
	if err != nil {
		result = "error"
		errMsg = err.Error()
		s.log.Warn("service action failed", "name", act.Name, "action", act.Op, "err", err)
	}
	s.broadcastServiceEvent(act.Name, act.Op, result, errMsg)
	return nil, nil
}

func sendMsgReqFromAction(act ws.ClientAction) (sendMsgReq, error) {
	var req sendMsgReq
	if len(act.Body) == 0 {
		return req, errors.New("empty")
	}
	if err := json.Unmarshal(act.Body, &req.Body); err != nil {
		req.Body = strings.TrimSpace(string(act.Body))
	}
	if len(act.Attachments) > 0 {
		if err := json.Unmarshal(act.Attachments, &req.Attachments); err != nil {
			return req, err
		}
	}
	return req, nil
}
