/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package engine

import (
	"fmt"
	"time"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/utils"
)

// NewStatsMetrics instantiates the StatsMetrics
// cfg serves as general purpose container to pass config options to metric
func NewStatsMetric(metricID string) (sm StatsMetric, err error) {
	metrics := map[string]func() (StatsMetric, error){
		utils.MetaASR: NewASR,
		utils.MetaACD: NewACD,
	}
	if _, has := metrics[metricID]; !has {
		return nil, fmt.Errorf("unsupported metric: %s", metricID)
	}
	return metrics[metricID]()
}

// StatsMetric is the interface which a metric should implement
type StatsMetric interface {
	GetValue() interface{}
	GetStringValue(fmtOpts string) (val string)
	GetFloat64Value() (val float64)
	AddEvent(ev StatsEvent) error
	RemEvent(ev StatsEvent) error
	GetMarshaled(ms Marshaler) (vals []byte, err error)
	SetFromMarshaled(vals []byte, ms Marshaler) (err error) // mostly used to load from DB
}

func NewASR() (StatsMetric, error) {
	return new(ASRStat), nil
}

// ASR implements AverageSuccessRatio metric
type ASRStat struct {
	Answered float64
	Count    float64
}

func (asr *ASRStat) GetValue() (v interface{}) {
	if asr.Count == 0 {
		return float64(STATS_NA)
	}
	return utils.Round((asr.Answered / asr.Count * 100),
		config.CgrConfig().RoundingDecimals, utils.ROUNDING_MIDDLE)
}

func (asr *ASRStat) GetStringValue(fmtOpts string) (valStr string) {
	if asr.Count == 0 {
		return utils.NOT_AVAILABLE
	}
	val := asr.GetValue().(float64)
	return fmt.Sprintf("%v%%", val) // %v will automatically limit the number of decimals printed
}

func (asr *ASRStat) GetFloat64Value() (val float64) {
	return asr.GetValue().(float64)
}

func (asr *ASRStat) AddEvent(ev StatsEvent) (err error) {
	if at, err := ev.AnswerTime(config.CgrConfig().DefaultTimezone); err != nil &&
		err != utils.ErrNotFound {
		return err
	} else if !at.IsZero() {
		asr.Answered += 1
	}
	asr.Count += 1
	return
}

func (asr *ASRStat) RemEvent(ev StatsEvent) (err error) {
	if at, err := ev.AnswerTime(config.CgrConfig().DefaultTimezone); err != nil &&
		err != utils.ErrNotFound {
		return err
	} else if !at.IsZero() {
		asr.Answered -= 1
	}
	asr.Count -= 1
	return
}

func (asr *ASRStat) GetMarshaled(ms Marshaler) (vals []byte, err error) {
	return ms.Marshal(asr)
}

func (asr *ASRStat) SetFromMarshaled(vals []byte, ms Marshaler) (err error) {
	return ms.Unmarshal(vals, asr)
}

func NewACD() (StatsMetric, error) {
	return new(ACDStat), nil
}

// ACD implements AverageCallDuration metric
type ACDStat struct {
	Sum   time.Duration
	Count int
}

func (acd *ACDStat) GetStringValue(fmtOpts string) (val string) {
	return
}

func (acd *ACDStat) GetValue() (v interface{}) {
	return
}

func (acd *ACDStat) GetFloat64Value() (v float64) {
	return float64(STATS_NA)
}

func (acd *ACDStat) AddEvent(ev StatsEvent) (err error) {
	return
}

func (acd *ACDStat) RemEvent(ev StatsEvent) (err error) {
	return
}

func (acd *ACDStat) GetMarshaled(ms Marshaler) (vals []byte, err error) {
	return
}

func (acd *ACDStat) SetFromMarshaled(vals []byte, ms Marshaler) (err error) {
	return
}