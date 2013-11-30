package protocol

type RequestMessage struct {
	Argv []string
}

type ResponseMessage struct{}

type ExitStatusMessage struct {
	ExitStatus int
}
