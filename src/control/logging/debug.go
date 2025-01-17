//
// (C) Copyright 2019 Intel Corporation.
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
//
// GOVERNMENT LICENSE RIGHTS-OPEN SOURCE SOFTWARE
// The Government's rights to use, modify, reproduce, release, perform, display,
// or disclose this software are subject to the terms of the Apache License as
// provided in Contract No. 8F-30005.
// Any reproduction of computer software, computer software documentation, or
// portions thereof marked with this legend must also reproduce the markings.
//
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
)

const debugLogFlags = log.Lmicroseconds | log.Lshortfile

// NewDebugLogger returns a DebugLogger configured for outputting
// debugging messages.
func NewDebugLogger(dest io.Writer) *DefaultDebugLogger {
	return &DefaultDebugLogger{
		baseLogger{
			dest: dest,
			log:  log.New(dest, "DEBUG ", debugLogFlags),
		},
	}
}

// DefaultDebugLogger implements the DebugLogger interface.
type DefaultDebugLogger struct {
	baseLogger
}

// Debugf emits a formatted debug message.
func (l *DefaultDebugLogger) Debugf(format string, args ...interface{}) {
	out := fmt.Sprintf(format, args...)
	if err := l.log.Output(logOutputDepth, out); err != nil {
		fmt.Fprintf(os.Stderr, "logger Debugf() failed: %s\n", err)
	}
}
