/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package webapi

import (
	"encoding/json"
	"net/http"

	"github.com/vogo/logtail"
	"github.com/vogo/logtail/transfer"
)

func routeToTransfer(runner *logtail.Runner, request *http.Request, response http.ResponseWriter, router string) {
	switch router {
	case OpTypes:
		listTransferTypes(runner, response)
	case OpList:
		listTransfers(runner, response)
	case OpAdd:
		addTransfer(runner, request, response)
	case OpDelete:
		deleteTransfer(runner, request, response)
	default:
		routeToNotFound(response)
	}
}

func listTransferTypes(_ *logtail.Runner, response http.ResponseWriter) {
	response.Header().Add("content-type", "application/json")

	// nolint:errchkjson //ignore this
	b, _ := json.Marshal(transfer.Types)

	_, _ = response.Write(b)
}

func listTransfers(runner *logtail.Runner, response http.ResponseWriter) {
	response.Header().Add("content-type", "application/json")

	// nolint:errchkjson //ignore this
	b, _ := json.Marshal(runner.Config.Transfers)

	_, _ = response.Write(b)
}

func addTransfer(runner *logtail.Runner, request *http.Request, response http.ResponseWriter) {
	config := &logtail.TransferConfig{}

	if err := json.NewDecoder(request.Body).Decode(config); err != nil {
		routeToError(response, err)

		return
	}

	if err := runner.AddTransfer(config); err != nil {
		routeToError(response, err)

		return
	}

	routeToSuccess(response)
}

func deleteTransfer(runner *logtail.Runner, request *http.Request, response http.ResponseWriter) {
	config := &logtail.TransferConfig{}

	if err := json.NewDecoder(request.Body).Decode(config); err != nil {
		routeToError(response, err)

		return
	}

	if err := runner.StopTransfer(config.Name); err != nil {
		routeToError(response, err)

		return
	}

	routeToSuccess(response)
}
