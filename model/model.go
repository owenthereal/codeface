package model

type EditorRequest struct {
	GitRepo string
}

type EditorResponse struct {
	URL string
}

type ErrorResponse struct {
	Error string
}
