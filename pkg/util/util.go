// Copyright 2024 Michael Vittrup Larsen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package util

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

func CsvToList(in string) []string {
	lst := strings.Split(in, ",")
	for idx, itm := range lst {
		lst[idx] = strings.TrimSpace(itm)
	}
	return lst
}

func LookupAuthSecret(secretName, namespace string, rl *fn.ResourceList) (username, password *string, err error) {
	if namespace == "" {
		namespace = "default" // Default according to spec
	}
	for _, k := range rl.Items {
		if !k.IsGVK("v1", "", "Secret") || k.GetName() != secretName {
			continue
		}
		oNamespace := k.GetNamespace()
		if oNamespace == "" {
			oNamespace = "default" // Default according to spec
		}
		if namespace == oNamespace {
			uname, found, err := k.NestedString("data", "username")
			if !found {
				return nil, nil, fmt.Errorf("key 'username' not found in Secret %s/%s", namespace, secretName)
			}
			if err != nil {
				return nil, nil, err
			}
			pword, found, err := k.NestedString("data", "password")
			if !found {
				return nil, nil, fmt.Errorf("key 'password' not found in Secret %s/%s", namespace, secretName)
			}
			if err != nil {
				return nil, nil, err
			}
			u, err := base64.StdEncoding.DecodeString(uname)
			if err != nil {
				return nil, nil, fmt.Errorf("decoding 'username' in Secret %s/%s: %w", namespace, secretName, err)
			}
			uname = string(u)
			p, err := base64.StdEncoding.DecodeString(pword)
			if err != nil {
				return nil, nil, fmt.Errorf("decoding 'password' in Secret %s/%s: %w", namespace, secretName, err)
			}
			pword = string(p)
			return &uname, &pword, nil
		}
	}
	return nil, nil, fmt.Errorf("auth Secret %s/%s not found", namespace, secretName)
}
