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
	"maps"
	"slices"
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

func ResultPrintf(fnResults *fn.Results, sev fn.Severity, format string, a ...any) {
	*fnResults = append(*fnResults,
		fn.GeneralResult(fmt.Sprintf(format+"\n", a...), sev))
}

// LookupAuthSecret will lookup a secret in a resourcelist and return username and password decoded from secret
func LookupAuthSecret(secretName, namespace string, rl *fn.ResourceList) (username, password string, err error) {
	return LookupAuthSecretWithKeys(secretName, namespace, "username", "password", rl)
}

// LookupSSHAuthSecret will lookup an SSH secret in a resourcelist and return username and password decoded from secret
func LookupSSHAuthSecret(secretName, namespace string, rl *fn.ResourceList) (username, password string, err error) {
	return LookupAuthSecretWithKeys(secretName, namespace, "ssh-username", "ssh-privatekey", rl)
}

// LookupAuthSecretWithKeys will lookup a secret in a resourcelist and return username and password decoded from secret with the username and password being defined by supplied key names
func LookupAuthSecretWithKeys(secretName, namespace, usernameKey, passwordKey string, rl *fn.ResourceList) (username, password string, err error) {
	if namespace == "" {
		namespace = "default" // Default according to spec
	}
	username = ""
	password = ""
	for _, k := range rl.Items {
		if !k.IsGVK("v1", "", "Secret") || k.GetName() != secretName {
			continue
		}
		oNamespace := k.GetNamespace()
		if oNamespace == "" {
			oNamespace = "default" // Default according to spec
		}
		var found bool
		if namespace == oNamespace {
			username, found, err = k.NestedString("data", usernameKey)
			if !found {
				err = fmt.Errorf("key '%v' not found in Secret %s/%s", usernameKey, namespace, secretName)
				return
			}
			if err != nil {
				return
			}
			password, found, err = k.NestedString("data", passwordKey)
			if !found {
				err = fmt.Errorf("key '%v' not found in Secret %s/%s", passwordKey, namespace, secretName)
				return
			}
			if err != nil {
				return
			}
			var u, p []byte
			u, err = base64.StdEncoding.DecodeString(username)
			if err != nil {
				err = fmt.Errorf("decoding '%v' in Secret %s/%s: %w", usernameKey, namespace, secretName, err)
				return
			}
			username = string(u)
			p, err = base64.StdEncoding.DecodeString(password)
			if err != nil {
				err = fmt.Errorf("decoding '%v' in Secret %s/%s: %w", passwordKey, namespace, secretName, err)
				return
			}
			password = string(p)
			return
		}
	}
	err = fmt.Errorf("auth Secret %s/%s not found", namespace, secretName)
	return
}

// UniqueStrings removes duplicate strings from slice
func UniqueStrings(list []string) []string {
	slices.Sort(list)
	return slices.Compact(list)
}

// MergeMaps will merge m1 and m2 with precedence to m2
func MergeMaps[M ~map[K]V, K comparable, V any](m1, m2 M) M {
	merged := make(M, len(m1) + len(m2))
	maps.Copy(merged, m1)
	maps.Copy(merged, m2)
	return merged
}
