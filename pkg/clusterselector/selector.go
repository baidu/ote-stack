/*
Copyright 2019 Baidu, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package clusterselector implements the routing of cluster messages.
package clusterselector

import (
	"regexp"
	"strings"
)

const (
	// SELECTOR_PATTERN_DELIMITER defines the delimiter of routing rules.
	SELECTOR_PATTERN_DELIMITER = ","
)

// Selector is the interface of cluster selector.
type Selector interface {
	// Has determines whether the name matchs the rules.
	Has(string) bool
}

type selector struct {
	pattern []string
}

// NewSelector returns a new selector object with given routing rules.
func NewSelector(s string) Selector {
	ps := strings.Split(s, SELECTOR_PATTERN_DELIMITER)
	for i := range ps {
		ps[i] = strings.TrimSpace(ps[i])
	}
	return &selector{ps}
}

func (s *selector) Has(clusterName string) bool {
	for _, p := range s.pattern {
		if ok, _ := regexp.MatchString(p, clusterName); ok {
			return true
		}
	}
	return false
}

// ClustersToSelector combines given clusters to routing rule.
func ClustersToSelector(clusters *[]string) string {
	return strings.Join(*clusters, SELECTOR_PATTERN_DELIMITER)
}
