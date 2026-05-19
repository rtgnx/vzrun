package v1

import "net/url"

const (
	VMsRoute       = "/v1/vms"
	VMRoute        = "/v1/vms/{name}"
	VMStartRoute   = "/v1/vms/{name}/start"
	VMStopRoute    = "/v1/vms/{name}/stop"
	VMRestartRoute = "/v1/vms/{name}/restart"
	VMLogsRoute    = "/v1/vms/{name}/logs"
	VMAttachRoute  = "/v1/vms/{name}/attach"
)

func VMPath(name string) string {
	return VMsRoute + "/" + url.PathEscape(name)
}

func VMStartPath(name string) string {
	return VMPath(name) + "/start"
}

func VMStopPath(name string) string {
	return VMPath(name) + "/stop"
}

func VMRestartPath(name string) string {
	return VMPath(name) + "/restart"
}

func VMLogsPath(name string) string {
	return VMPath(name) + "/logs"
}

func VMAttachPath(name string) string {
	return VMPath(name) + "/attach"
}
