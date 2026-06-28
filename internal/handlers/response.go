package handlers

// SeaResponse is a generic response type for the API.
// It is used to return data or errors.
type SeaResponse[R any] struct {
	Error   string `json:"error,omitempty"`
	Details string `json:"details,omitempty"` // Additional diagnostic details for debugging
	Data    R      `json:"data,omitempty"`
}

func NewDataResponse[R any](data R) SeaResponse[R] {
	res := SeaResponse[R]{
		Data: data,
	}
	return res
}

func NewErrorResponse(err error) SeaResponse[any] {
	if err == nil {
		return SeaResponse[any]{
			Error: "Unknown error",
		}
	}
	res := SeaResponse[any]{
		Error: err.Error(),
	}
	return res
}

func NewDetailedErrorResponse(err error, details string) SeaResponse[any] {
	if err == nil {
		return SeaResponse[any]{
			Error: "Unknown error",
		}
	}
	return SeaResponse[any]{
		Error:   err.Error(),
		Details: details,
	}
}
