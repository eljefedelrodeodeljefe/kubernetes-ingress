// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"log"
	"sort"

	"github.com/haproxytech/models"
)

type BackendSwitchingRule struct {
	Host    string
	Path    string
	Backend string
}

func (c *HAProxyController) useBackendRuleRefresh() (needsReload bool) {
	needsReload = false
	if c.cfg.UseBackendRulesStatus == EMPTY {
		return needsReload
	}
	frontends := []string{FrontendHTTP, FrontendHTTPS}

	nativeAPI := c.NativeAPI

	sortedList := []string{}
	for name := range c.cfg.UseBackendRules {
		sortedList = append(sortedList, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedList))) // reverse order

	_, frontend, _ := c.cfg.NativeAPI.Configuration.GetFrontend(FrontendHTTPS, c.ActiveTransaction)
	backends := map[string]struct{}{
		frontend.DefaultBackend: struct{}{},
		"RateLimit":             struct{}{},
	}
	for _, frontend := range frontends {
		var err error
		for err == nil {
			err = nativeAPI.Configuration.DeleteBackendSwitchingRule(0, frontend, c.ActiveTransaction, 0)
		}
		for _, name := range sortedList {
			rule := c.cfg.UseBackendRules[name]
			id := int64(0)
			var condTest string
			if rule.Host != "" {
				condTest = fmt.Sprintf("{ req.hdr(host) -i %s } ", rule.Host)
			}
			if rule.Path != "" {
				condTest = fmt.Sprintf("%s{ path_beg %s }", condTest, rule.Path)
			}
			if condTest == "" {
				log.Println(fmt.Sprintf("Both Host and Path are empty for frontend %s with backend %s, SKIP", frontend, rule.Backend))
				continue
			}
			backendSwitchingRule := &models.BackendSwitchingRule{
				Cond:     "if",
				CondTest: condTest,
				Name:     rule.Backend,
				ID:       &id,
			}
			backends[rule.Backend] = struct{}{}
			err = c.cfg.NativeAPI.Configuration.CreateBackendSwitchingRule(frontend, backendSwitchingRule, c.ActiveTransaction, 0)
			LogErr(err)
		}
	}
	_, allBackends, _ := c.cfg.NativeAPI.Configuration.GetBackends(c.ActiveTransaction)
	for _, backend := range allBackends {
		_, ok := backends[backend.Name]
		if !ok {
			err := nativeAPI.Configuration.DeleteBackend(backend.Name, c.ActiveTransaction, 0)
			LogErr(err)
		}
	}

	if c.cfg.UseBackendRulesStatus != EMPTY {
		needsReload = true
	}

	return needsReload
}
