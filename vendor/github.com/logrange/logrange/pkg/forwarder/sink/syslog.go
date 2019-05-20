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

package sink

import (
	"fmt"
	"github.com/logrange/logrange/api"
	"github.com/logrange/logrange/pkg/model"
	"github.com/logrange/logrange/pkg/model/field"
	"github.com/logrange/logrange/pkg/syslog"
	"github.com/logrange/logrange/pkg/utils"
	"github.com/logrange/range/pkg/utils/bytes"
	"github.com/mitchellh/mapstructure"
	"time"
)

type (

	//literal or {time}, {message}, {tags}, {vars:key}
	syslogMessageSchemaCfg struct {
		Facility string
		Severity string
		Hostname string
		Tags     string
		Msg      string
	}

	syslogMessageSchema struct {
		facility *model.FormatParser
		severity *model.FormatParser
		hostname *model.FormatParser
		tags     *model.FormatParser
		msg      *model.FormatParser
	}

	syslogSinkConfig struct {
		syslog.Config `mapstructure:",squash"`
		MessageSchema *syslogMessageSchemaCfg
	}

	syslogSink struct {
		slog *syslog.Logger
		schm *syslogMessageSchema
	}
)

//===================== syslogSink =====================

func newSyslogSink(cfg *syslogSinkConfig) (*syslogSink, error) {
	ms, err := cfg.GetSyslogMsgSchema()
	if err == nil {
		slog, err := syslog.NewLogger(&cfg.Config)
		if err == nil {
			return &syslogSink{
				slog: slog,
				schm: ms,
			}, nil
		}
	}
	return nil, err
}

// OnEvent sends the events via syslog to the destination
func (ss *syslogSink) OnEvent(events []*api.LogEvent) error {
	var (
		me model.LogEvent
		sm syslog.Message
	)

	for _, e := range events {
		copyEv(e, &me)
		ss.schm.format(&me, e.Tags, &sm)
		err := ss.slog.Write(&sm)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ss *syslogSink) Close() error {
	if ss.slog != nil {
		return ss.slog.Close()
	}
	return nil
}

func copyEv(e *api.LogEvent, me *model.LogEvent) {
	me.Timestamp = e.Timestamp
	me.Msg = bytes.StringToByteArray(e.Message)
	me.Fields = field.Parse(e.Fields)
}

//===================== syslogSinkConfig =====================

func newSyslogSinkConfig(params Params) (*syslogSinkConfig, error) {
	cfg := &syslogSinkConfig{}
	if params == nil || len(params) == 0 {
		return cfg, nil
	}
	if err := mapstructure.Decode(params, cfg); err != nil {
		return nil, fmt.Errorf("unable to decode Params=%v; %v", params, err)
	}
	return cfg, nil
}

func (ss *syslogSinkConfig) Check() error {
	err := ss.Config.Check()
	if err != nil {
		return err
	}
	if ss.MessageSchema != nil {
		_, err := ss.GetSyslogMsgSchema()
		if err != nil {
			return fmt.Errorf("invalid MessageSchema=%v; %v", ss.MessageSchema, err)
		}
	}
	return nil
}

func (ss *syslogSinkConfig) GetSyslogMsgSchema() (*syslogMessageSchema, error) {
	var (
		err error
		ms  syslogMessageSchema
	)

	msCfg := ss.MessageSchema
	if msCfg != nil {
		if msCfg.Facility != "" {
			if ms.facility, err = model.NewFormatParser(msCfg.Facility); err != nil {
				return nil, err
			}
		}
		if msCfg.Severity != "" {
			if ms.severity, err = model.NewFormatParser(msCfg.Severity); err != nil {
				return nil, err
			}
		}
		if msCfg.Hostname != "" {
			if ms.hostname, err = model.NewFormatParser(msCfg.Hostname); err != nil {
				return nil, err
			}
		}
		if msCfg.Tags != "" {
			if ms.tags, err = model.NewFormatParser(msCfg.Tags); err != nil {
				return nil, err
			}
		}
		if msCfg.Msg != "" {
			if ms.msg, err = model.NewFormatParser(msCfg.Msg); err != nil {
				return nil, err
			}
		}
	}
	return &ms, nil
}

//===================== syslogMessageSchema =====================

func (s *syslogMessageSchema) format(me *model.LogEvent, tags string, sm *syslog.Message) {
	sm.Facility = syslog.FacilityLocal6
	if s.facility != nil {
		f, err := syslog.Facility(s.facility.FormatStr(me, tags))
		if err == nil {
			sm.Facility = f
		}
	}
	sm.Severity = syslog.SeverityInfo
	if s.severity != nil {
		s, err := syslog.Severity(s.severity.FormatStr(me, tags))
		if err == nil {
			sm.Severity = s
		}
	}

	sm.Time = time.Unix(0, int64(me.Timestamp))
	sm.Hostname = "localhost"
	if s.hostname != nil {
		sm.Hostname = s.hostname.FormatStr(me, tags)
	}
	sm.Tag = tags
	if s.tags != nil {
		sm.Tag = s.tags.FormatStr(me, tags)
	}
	sm.Msg = bytes.ByteArrayToString(me.Msg)
	if s.msg != nil {
		sm.Msg = s.msg.FormatStr(me, tags)
	}
}

//===================== syslogMessageSchemaCfg =====================

func (cfg *syslogMessageSchemaCfg) String() string {
	return utils.ToJsonStr(cfg)
}
