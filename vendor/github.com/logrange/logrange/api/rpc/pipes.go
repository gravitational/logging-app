// Copyright 2018-2019 The logrange Authors
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

package rpc

import (
	"context"
	"encoding/json"
	"github.com/jrivets/log4g"
	"github.com/logrange/logrange/api"
	"github.com/logrange/logrange/pkg/pipe"
	"github.com/logrange/range/pkg/records"
	rrpc "github.com/logrange/range/pkg/rpc"
	"github.com/pkg/errors"
)

type (
	// ServerAdmin implements server part of api.Admin interface
	ServerPipes struct {
		PipeService *pipe.Service `inject:""`

		logger log4g.Logger
	}

	clntPipes struct {
		rc rrpc.Client
	}
)

func (cp *clntPipes) EnsurePipe(ctx context.Context, p api.Pipe, res *api.PipeCreateResult) error {
	buf, err := json.Marshal(p)
	if err != nil {
		return errors.Wrapf(err, "could not marshal request ")
	}

	resp, opErr, err := cp.rc.Call(ctx, cRpcEpPipesEnsure, records.Record(buf))
	if err != nil {
		return errors.Wrapf(err, "could not sent request via rpc")
	}

	if opErr == nil {
		err = json.Unmarshal(resp, res)
	}

	res.Err = opErr
	cp.rc.Collect(resp)

	return err
}

func NewServerPipes() *ServerPipes {
	sp := new(ServerPipes)
	sp.logger = log4g.GetLogger("rpc.pipes")
	return sp
}

func (sp *ServerPipes) ensurePipe(reqId int32, reqBody []byte, sc *rrpc.ServerConn) {
	var req api.Pipe
	err := json.Unmarshal(reqBody, &req)
	if err != nil {
		sp.logger.Error("ensurePipe(): could not unmarshal the body request ")
		sc.SendResponse(reqId, err, cEmptyResponse)
		return
	}

	sst := pipe.Pipe{req.Name, req.TagsCond, req.FilterCond}
	dst, err := sp.PipeService.EnsurePipe(sst)
	if err != nil {
		sp.logger.Warn("ensurePipe(): got the err=", err)
		sc.SendResponse(reqId, err, cEmptyResponse)
		return
	}

	req.TagsCond = dst.TagsCond
	req.FilterCond = dst.FltCond
	req.Destination = dst.DestTags.Line().String()

	res := api.PipeCreateResult{Pipe: req, Err: nil}
	buf, err := json.Marshal(res)
	if err != nil {
		sp.logger.Warn("ensurePipe(): could not marshal result err=", err)
		sc.SendResponse(reqId, err, cEmptyResponse)
		return
	}

	sc.SendResponse(reqId, nil, records.Record(buf))
}
