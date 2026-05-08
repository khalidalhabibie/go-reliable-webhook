package httpresponse

type Response struct {
	Data any `json:"data"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func OK(data any) Response {
	return Response{Data: data}
}

func Error(message string) ErrorResponse {
	return ErrorResponse{Error: message}
}
