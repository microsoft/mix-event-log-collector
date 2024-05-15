package cli

import "fmt"

type ShowOffsetsCmd struct {
}

type ResetOffsetsCmd struct {
}

type SetOffsetCmd struct {
	Partition int    `arg:"env:ELC_KAFKA_PARTITION,-p,--partition" default:"0" help:"Kafka partition on which to set the offset"`
	Offset    int    `arg:"env:ELC_KAFKA_OFFSET,-o,--offset" default:"0" help:"Offset value to set the partition position to"`
	AppID     string `arg:"env:ELC_APP_ID,-a,--appID,required" help:"The Mix 3 App ID in the config file to filter on"`
}

type args struct {
	LoggingToFileEnabled bool   `arg:"env:ELC_ENABLE_LOGGING_TO_FILE,-l,--logToFile" default:"false" help:"Enable logging to file"`
	LogfilePath          string `arg:"env:ELC_LOGFILE_PATH,-f,--logfilePath" default:"logs/event-log-collector.log" help:"Location and name of file to log to"`
	DebugEnabled         bool   `arg:"env:ELC_ENABLE_DEBUG_LOGGING,-d,--debug" help:"Specify this flag to enable debug logging level"`
	Config               string `arg:"env:ELC_CONFIG,-c,--config" default:"configs/" help:"Path to event logger configs. Can be either a directory or specific json config file"`

	Port               int    `arg:"env:ELC_PORT,-p,--port" default:"8078" help:"Port for the service to listen on"`
	Addr               string `arg:"env:ELC_ADDRESS,-a,--addr" default:"localhost" help:"Address of the service"`
	Scheme             string `arg:"env:ELC_SCHEME,-s,--scheme" default:"http" help:"Scheme of the service (http or https"`
	ServerReadTimeout  int    `arg:"env:ELC_SERVER_READ_TIMEOUT,--serverReadTimeout" default:"60" help:"Server read timeout in seconds"`
	ServerWriteTimeout int    `arg:"env:ELC_SERVER_WRITE_TIMEOUT,--serverWriteTimeout" default:"60" help:"Server write timeout in seconds"`
	BasePath           string `arg:"env:ELC_BASE_PATH,--basePath" default:"" help:"Base path to prefix api routes. Use this when deployed behind a reverse proxy"`
}

// go-args library supports a Description() method on the struct to print out a description of the command
func (args) Description() string {
	docsRoot := "https://white-mud-08c17b10f.1.azurestaticapps.net"

	description := `
#########################################################################################################
For additional details on configuring event log collection, refer to the online docs: 

	%s
	
For a list of available environment vars that can be used to configure event log collector, refer to: 
	
	%s/docs/configuration-guide/#environment-vars

	e.g. ELC_CLIENT_ID=<mix client id> ELC_CLIENT_SECRET=<mix client secret> ./event-log-client
	
For a quick start guide to get up and running quickly, check out: 

	%s/docs/getting-started/
#########################################################################################################
`

	return fmt.Sprintf(description, docsRoot, docsRoot, docsRoot)
}

// Export Args
var Args args
