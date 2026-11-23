package core

const MaxPage int32 = 21474836

type PageRequest struct {
	Page  int32 `binding:"omitempty,min=1,max=21474836" form:"page" json:"page"`
	Limit int32 `binding:"omitempty,min=1,max=100" form:"limit" json:"limit"`
}

func (p PageRequest) Values() (page, limit, offset int32) {
	page, limit = p.Page, p.Limit
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 20
	}
	return page, limit, (page - 1) * limit
}

type Page[T any] struct {
	Items []T   `json:"items"`
	Page  int32 `json:"page"`
	Limit int32 `json:"limit"`
	Total int64 `json:"total"`
}

type ActorRequest struct {
	UpdatedBy string `binding:"required" example:"admin" json:"updatedBy"`
}

// MapPage builds a page of responses from stored items.
func MapPage[S, T any](items []S, page, limit int32, total int64, convert func(S) T) Page[T] {
	result := Page[T]{Items: make([]T, len(items)), Page: page, Limit: limit, Total: total}
	for i, item := range items {
		result.Items[i] = convert(item)
	}
	return result
}
