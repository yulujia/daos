//
// (C) Copyright 2018-2019 Intel Corporation.
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

package main

import (
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"

	flags "github.com/jessevdk/go-flags"
	"github.com/pkg/errors"

	"github.com/daos-stack/daos/src/control/client"
	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/drpc"
	"github.com/daos-stack/daos/src/control/logging"
)

const (
	agentSockName        = "agent.sock"
	daosAgentDrpcSockEnv = "DAOS_AGENT_DRPC_DIR"
)

type cliOptions struct {
	Debug      bool   `short:"d" long:"debug" description:"Enable debug output"`
	JSON       bool   `short:"j" long:"json" description:"Enable JSON output"`
	ConfigPath string `short:"o" long:"config-path" description:"Path to agent configuration file" default:"etc/daos_agent.yml"`
	Insecure   bool   `short:"i" long:"insecure" description:"have agent attempt to connect without certificates"`
	RuntimeDir string `short:"s" long:"runtime_dir" description:"Path to agent communications socket"`
	LogFile    string `short:"l" long:"logfile" description:"Full path and filename for daos agent log file"`
}

func exitWithError(log logging.Logger, err error) {
	log.Errorf("%s: %v", path.Base(os.Args[0]), err)
	os.Exit(1)
}

func main() {
	var opts cliOptions
	log := logging.NewCommandLineLogger()

	if err := agentMain(log, &opts); err != nil {
		exitWithError(log, err)
	}
}

// applyCmdLineOverrides will overwrite Configuration values with any non empty
// data provided, usually from the commandline.
func applyCmdLineOverrides(log logging.Logger, c *client.Configuration, opts *cliOptions) {

	if opts.RuntimeDir != "" {
		log.Debugf("Overriding socket path from config file with %s", opts.RuntimeDir)
		c.RuntimeDir = opts.RuntimeDir
	}

	if opts.LogFile != "" {
		log.Debugf("Overriding LogFile path from config file with %s", opts.LogFile)
		c.LogFile = opts.LogFile
	}
	if opts.Insecure == true {
		log.Debugf("Overriding AllowInsecure from config file with %t", opts.Insecure)
		c.TransportConfig.AllowInsecure = true
	}
}

func agentMain(log *logging.LeveledLogger, opts *cliOptions) error {
	log.Info("Starting daos_agent:")

	p := flags.NewParser(opts, flags.Default)

	_, err := p.Parse()
	if err != nil {
		return err
	}

	if opts.JSON {
		log.WithJSONOutput()
	}

	if opts.Debug {
		log.WithLogLevel(logging.LogLevelDebug)
		log.Debug("debug output enabled")
	}

	// Load the configuration file using the supplied path or the
	// default path if none provided.
	config, err := client.GetConfig(log, opts.ConfigPath)
	if err != nil {
		log.Errorf("An unrecoverable error occurred while processing the configuration file: %s", err)
		return err
	}

	// Override configuration with any commandline values given
	applyCmdLineOverrides(log, config, opts)

	env := config.Ext.Getenv(daosAgentDrpcSockEnv)
	if env != config.RuntimeDir {
		log.Debugf("Environment variable '%s' has value '%s' which does not "+
			"match '%s'", daosAgentDrpcSockEnv, env, config.RuntimeDir)
	}

	sockPath := filepath.Join(config.RuntimeDir, agentSockName)
	log.Debugf("Full socket path is now: %s", sockPath)

	if config.LogFile != "" {
		f, err := common.AppendFile(config.LogFile)
		if err != nil {
			log.Errorf("Failure creating log file: %s", err)
			return err
		}
		defer f.Close()
		log.Infof("Using logfile: %s", config.LogFile)

		// Create an additional set of loggers which append everything
		// to the specified file.
		log.WithErrorLogger(logging.NewErrorLogger("agent", f)).
			WithInfoLogger(logging.NewInfoLogger("agent", f)).
			WithDebugLogger(logging.NewDebugLogger(f))
	}

	err = config.TransportConfig.PreLoadCertData()
	if err != nil {
		return errors.Wrap(err, "Unable to load Cerificate Data")
	}

	// Setup signal handlers so we can block till we get SIGINT or SIGTERM
	signals := make(chan os.Signal, 1)
	finish := make(chan bool, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	drpcServer, err := drpc.NewDomainSocketServer(sockPath)
	if err != nil {
		log.Errorf("Unable to create socket server: %v", err)
		return err
	}

	drpcServer.RegisterRPCModule(NewSecurityModule(config.TransportConfig))
	drpcServer.RegisterRPCModule(&mgmtModule{config.AccessPoints[0], config.TransportConfig})

	err = drpcServer.Start()
	if err != nil {
		log.Errorf("Unable to start socket server on %s: %v", sockPath, err)
		return err
	}

	log.Infof("Listening on %s", sockPath)

	// Anonymous goroutine to wait on the signals channel and tell the
	// program to finish when it receives a signal. Since we only notify on
	// SIGINT and SIGTERM we should only catch this on a kill or ctrl+c
	// The syntax looks odd but <- Channel means wait on any input on the
	// channel.
	go func() {
		<-signals
		finish <- true
	}()
	<-finish
	drpcServer.Shutdown()

	return nil
}
